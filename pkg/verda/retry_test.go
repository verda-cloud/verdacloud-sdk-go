// Copyright 2026 Verda Cloud Oy
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package verda

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

const retryTestPath = "/retry-test"

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// syncLogger records log lines and is safe for concurrent use.
type syncLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *syncLogger) record(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(msg, args...))
}
func (l *syncLogger) Debug(msg string, args ...any) { l.record(msg, args...) }
func (l *syncLogger) Info(msg string, args ...any)  { l.record(msg, args...) }
func (l *syncLogger) Warn(msg string, args ...any)  { l.record(msg, args...) }
func (l *syncLogger) Error(msg string, args ...any) { l.record(msg, args...) }
func (l *syncLogger) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

// newRetryClient builds a client against a mock server and installs the retry
// middleware when retries > 0. The handler serves retryTestPath for every method,
// and hits counts the requests that reached it.
func newRetryClient(t *testing.T, retries int, handler func(hit int, w http.ResponseWriter, r *http.Request), opts ...ClientOption) (*Client, *atomic.Int32) {
	t.Helper()
	return newRetryClientAt(t, retryTestPath, retries, handler, opts...)
}

// newRetryClientAt is newRetryClient with the handler served at path.
func newRetryClientAt(t *testing.T, path string, retries int, handler func(hit int, w http.ResponseWriter, r *http.Request), opts ...ClientOption) (*Client, *atomic.Int32) {
	t.Helper()
	ms := testutil.NewMockServer()
	t.Cleanup(ms.Close)

	var hits atomic.Int32
	h := func(w http.ResponseWriter, r *http.Request) {
		handler(int(hits.Add(1)), w, r)
	}
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		ms.SetHandler(m, path, h)
	}

	cfg := testutil.NewTestClientConfig(ms)
	all := append([]ClientOption{
		WithBaseURL(cfg.BaseURL), WithClientID(cfg.ClientID), WithClientSecret(cfg.ClientSecret),
	}, opts...)
	client, err := NewClient(all...)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	if retries > 0 {
		client.AddRequestMiddleware(ExponentialBackoffRetryMiddleware(retries, time.Millisecond, &NoOpLogger{}))
	}
	return client, &hits
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"code":"err","message":%q}`, msg)
}

func doRetryRequest(t *testing.T, c *Client, ctx context.Context, method string, body []byte, out any) (*Response, error) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := c.NewRequest(ctx, method, retryTestPath, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return c.Do(req, out)
}

func TestDo_WithoutRetryMiddleware(t *testing.T) {
	t.Run("error status returns raw APIError after one attempt", func(t *testing.T) {
		client, hits := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, http.StatusNotFound, "missing")
		})
		resp, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
		apiErr, ok := err.(*APIError) //nolint:errorlint // callers type-assert the raw error
		if !ok {
			t.Fatalf("expected raw *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusNotFound || apiErr.Message != "missing" {
			t.Errorf("unexpected error %#v", apiErr)
		}
		if resp == nil || resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected the response alongside the error, got %v", resp)
		}
		if got := hits.Load(); got != 1 {
			t.Errorf("expected 1 request, got %d", got)
		}
	})

	t.Run("success decodes the body", func(t *testing.T) {
		client, hits := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"name":"ok"}`)
		})
		var out struct{ Name string }
		if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, &out); err != nil {
			t.Fatal(err)
		}
		if out.Name != "ok" || hits.Load() != 1 {
			t.Errorf("name=%q hits=%d", out.Name, hits.Load())
		}
	})
}

func TestDo_ResponseMiddlewareRunsOnce(t *testing.T) {
	client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusBadRequest, "nope")
	})
	var runs atomic.Int32
	client.AddResponseMiddleware(func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			runs.Add(1)
			return next(ctx)
		}
	})
	_, _ = doRetryRequest(t, client, context.Background(), http.MethodPost, []byte(`{}`), nil) //nolint:errcheck // error asserted elsewhere
	if got := runs.Load(); got != 1 {
		t.Errorf("expected response middleware once, got %d", got)
	}
}

func TestDo_LoggingMiddlewareSeesRealError(t *testing.T) {
	logger := &syncLogger{}
	client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusConflict, "already exists")
	}, WithLogger(logger))
	_, _ = doRetryRequest(t, client, context.Background(), http.MethodPost, []byte(`{}`), nil) //nolint:errcheck // error asserted elsewhere
	if out := logger.text(); !strings.Contains(out, "failed after") || !strings.Contains(out, "API error 409") {
		t.Errorf("expected LoggingMiddleware to log the real failure, got:\n%s", out)
	}
}

func TestDo_MiddlewareSkippingNextStillSends(t *testing.T) {
	client, hits := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{}`)
	})
	client.AddRequestMiddleware(func(RequestHandler) RequestHandler {
		return func(*RequestContext) error { return nil } // never calls next
	})
	if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("expected 1 request, got %d", hits.Load())
	}
}

func TestDo_MiddlewareFailureBeforeSend(t *testing.T) {
	client, hits := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {})
	boom := errors.New("boom")
	client.AddRequestMiddleware(func(RequestHandler) RequestHandler {
		return func(*RequestContext) error { return boom }
	})
	_, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "request middleware failed") {
		t.Errorf("unexpected error: %v", err)
	}
	if hits.Load() != 0 {
		t.Errorf("expected no request, got %d", hits.Load())
	}
}

// failingRT fails requests to retryTestPath and passes everything else (OAuth) through.
func failingRT(fail func(*http.Request) (*http.Response, error)) http.RoundTripper {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == retryTestPath {
			return fail(r)
		}
		return http.DefaultTransport.RoundTrip(r)
	})
}

func TestDo_MiddlewareSwallowsTransportError(t *testing.T) {
	transportErr := errors.New("link down")
	client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {},
		WithHTTPClient(&http.Client{Transport: failingRT(func(*http.Request) (*http.Response, error) {
			return nil, transportErr
		})}))
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			_ = next(ctx) //nolint:errcheck // this middleware swallows the error on purpose
			return nil
		}
	})
	resp, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
	if !errors.Is(err, transportErr) || !strings.Contains(err.Error(), "request failed") {
		t.Errorf("expected the transport error, got %v", err)
	}
}

func TestDo_HeaderDeletedByMiddlewareIsNotSent(t *testing.T) {
	var got http.Header
	client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		fmt.Fprint(w, `{}`)
	})
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Headers.Del("User-Agent")
			ctx.Headers.Set("X-Added", "yes")
			return next(ctx)
		}
	})
	if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got.Get("X-Added") != "yes" {
		t.Errorf("middleware header missing: %v", got)
	}
	if ua := got.Get("User-Agent"); strings.Contains(ua, "verda") {
		t.Errorf("deleted User-Agent was still sent: %q", ua)
	}
}

// noKeepAliveClient makes every request use a fresh connection, so the
// transport never retries a request on its own after a dropped connection.
func noKeepAliveClient() *http.Client {
	return &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
}

func abortConnection() { panic(http.ErrAbortHandler) }

func TestRetry_ReadMethodsRetryServerErrors(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions, ""} {
		t.Run("method="+method, func(t *testing.T) {
			client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, _ *http.Request) {
				if hit == 1 {
					writeAPIError(w, http.StatusServiceUnavailable, "try later")
					return
				}
				fmt.Fprint(w, `{"name":"ok"}`)
			})
			req, err := client.NewRequest(context.Background(), http.MethodGet, retryTestPath, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Method = method // the empty method is sent as GET
			var out struct{ Name string }
			if _, err := client.Do(req, &out); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hits.Load() != 2 {
				t.Errorf("expected 2 attempts, got %d", hits.Load())
			}
			if method != http.MethodHead && method != http.MethodOptions && out.Name != "ok" {
				t.Errorf("result not decoded: %+v", out)
			}
		})
	}
}

func TestRetry_ReadRetriesStatuses(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, 599} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client, hits := newRetryClient(t, 2, func(_ int, w http.ResponseWriter, _ *http.Request) {
				writeAPIError(w, status, "no")
			})
			_, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
			apiErr, ok := err.(*APIError) //nolint:errorlint // exhausted retries return the raw error
			if !ok || apiErr.StatusCode != status {
				t.Fatalf("expected raw *APIError %d, got %T: %v", status, err, err)
			}
			if hits.Load() != 3 {
				t.Errorf("expected 3 attempts, got %d", hits.Load())
			}
		})
	}
	t.Run("404 is not retried", func(t *testing.T) {
		client, hits := newRetryClient(t, 2, func(_ int, w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, http.StatusNotFound, "no")
		})
		_, _ = doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil) //nolint:errcheck // asserted via hits
		if hits.Load() != 1 {
			t.Errorf("expected 1 attempt, got %d", hits.Load())
		}
	})
}

func TestRetry_ReadTransportErrors(t *testing.T) {
	t.Run("dropped connection (EOF)", func(t *testing.T) {
		client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, _ *http.Request) {
			if hit == 1 {
				abortConnection()
			}
			fmt.Fprint(w, `{}`)
		}, WithHTTPClient(noKeepAliveClient()))
		if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hits.Load() != 2 {
			t.Errorf("expected 2 attempts, got %d", hits.Load())
		}
	})

	t.Run("truncated 200 body", func(t *testing.T) {
		client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, _ *http.Request) {
			if hit == 1 {
				w.Header().Set("Content-Length", "100")
				fmt.Fprint(w, `{"na`)
				w.(http.Flusher).Flush()
				abortConnection()
			}
			fmt.Fprint(w, `{"name":"ok"}`)
		}, WithHTTPClient(noKeepAliveClient()))
		var out struct{ Name string }
		if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hits.Load() != 2 || out.Name != "ok" {
			t.Errorf("hits=%d out=%+v", hits.Load(), out)
		}
	})

	t.Run("last attempt fails in transport: no stale response", func(t *testing.T) {
		var calls atomic.Int32
		linkDown := errors.New("link down")
		client, _ := newRetryClient(t, 1, func(int, http.ResponseWriter, *http.Request) {},
			WithHTTPClient(&http.Client{Transport: failingRT(func(r *http.Request) (*http.Response, error) {
				if calls.Add(1) == 1 {
					return &http.Response{
						StatusCode: http.StatusServiceUnavailable, Status: "503 Service Unavailable",
						Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"message":"busy"}`)), Request: r,
					}, nil
				}
				return nil, linkDown
			})}))
		resp, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
		if resp != nil {
			t.Errorf("expected nil response, got status %d", resp.StatusCode)
		}
		if !errors.Is(err, linkDown) {
			t.Errorf("expected the transport error, got %v", err)
		}
		if calls.Load() != 2 {
			t.Errorf("expected 2 attempts, got %d", calls.Load())
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name, value string
		want        time.Duration
	}{
		{"seconds", "2", 2 * time.Second},
		{"zero", "0", 0},
		{"padded", " 3 ", 3 * time.Second},
		{"future date", now.Add(5 * time.Second).Format(http.TimeFormat), 5 * time.Second},
		{"past date", now.Add(-time.Hour).Format(http.TimeFormat), 0},
		{"far future date is capped", now.Add(time.Hour).Format(http.TimeFormat), maxRetryDelay},
		{"huge seconds are capped", "999999999", maxRetryDelay},
		{"overflowing seconds are capped", "99999999999999999999999", maxRetryDelay},
		{"negative", "-5", 0},
		{"malformed", "soon", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRetryAfter(tt.value, now); got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestRetry_RetryAfterHeaderIsHonored(t *testing.T) {
	if testing.Short() {
		t.Skip("waits about one second")
	}
	client, hits := newRetryClient(t, 1, func(hit int, w http.ResponseWriter, _ *http.Request) {
		if hit == 1 {
			w.Header().Set("Retry-After", "1")
			writeAPIError(w, http.StatusServiceUnavailable, "busy")
			return
		}
		fmt.Fprint(w, `{}`)
	})
	start := time.Now()
	if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond || hits.Load() != 2 {
		t.Errorf("expected to wait about 1s for 2 attempts, waited %v for %d", elapsed, hits.Load())
	}
}

func TestRetry_RetryAfterIgnoredOnOtherStatuses(t *testing.T) {
	if got := retryAfterDelay(&attemptError{apiErr: &APIError{StatusCode: 500}, header: http.Header{"Retry-After": {"20"}}}); got != 0 {
		t.Errorf("Retry-After on 500 should be ignored, got %v", got)
	}
	if got := retryAfterDelay(&attemptError{apiErr: &APIError{StatusCode: 429}, header: http.Header{"Retry-After": {"20"}}}); got != 20*time.Second {
		t.Errorf("Retry-After on 429 = %v, want 20s", got)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	slowRetryClient := func(t *testing.T, handler func(int, http.ResponseWriter, *http.Request)) (*Client, *atomic.Int32) {
		client, hits := newRetryClient(t, 0, handler)
		client.AddRequestMiddleware(ExponentialBackoffRetryMiddleware(3, 10*time.Second, &NoOpLogger{}))
		return client, hits
	}

	t.Run("canceled during backoff", func(t *testing.T) {
		client, hits := slowRetryClient(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, http.StatusServiceUnavailable, "busy")
		})
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := doRetryRequest(t, client, ctx, http.MethodGet, nil, nil)
		var apiErr *APIError
		if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &apiErr) {
			t.Errorf("expected deadline and API error to be wrapped, got %v", err)
		}
		if hits.Load() != 1 || time.Since(start) > 5*time.Second {
			t.Errorf("hits=%d after %v", hits.Load(), time.Since(start))
		}
	})

	t.Run("canceled right after a 503", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client, hits := slowRetryClient(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
			cancel()
			writeAPIError(w, http.StatusServiceUnavailable, "busy")
		})
		_, err := doRetryRequest(t, client, ctx, http.MethodGet, nil, nil)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected cancellation, got %v", err)
		}
		if hits.Load() != 1 {
			t.Errorf("expected no further attempt, got %d", hits.Load())
		}
	})
}

// trackedBodies hands out replay bodies and counts how often each is closed.
type trackedBodies struct {
	mu     sync.Mutex
	bodies []*trackedBody
}

type trackedBody struct {
	io.Reader
	closes atomic.Int32
}

func (b *trackedBody) Close() error { b.closes.Add(1); return nil }

func (tb *trackedBodies) open(data []byte) io.ReadCloser {
	b := &trackedBody{Reader: bytes.NewReader(data)}
	tb.mu.Lock()
	tb.bodies = append(tb.bodies, b)
	tb.mu.Unlock()
	return b
}

// assertAllClosedOnce fails unless every replay body was closed exactly once.
func (tb *trackedBodies) assertAllClosedOnce(t *testing.T, wantOpened int) {
	t.Helper()
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if len(tb.bodies) != wantOpened {
		t.Errorf("expected %d replay bodies opened, got %d", wantOpened, len(tb.bodies))
	}
	for i, b := range tb.bodies {
		if got := b.closes.Load(); got != 1 {
			t.Errorf("replay body %d closed %d times, want 1", i, got)
		}
	}
}

func newTrackedRequest(t *testing.T, c *Client, method string, data []byte) (*http.Request, *trackedBodies) {
	t.Helper()
	req, err := c.NewRequest(context.Background(), method, retryTestPath, io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}
	tracked := &trackedBodies{}
	req.GetBody = func() (io.ReadCloser, error) { return tracked.open(data), nil }
	req.ContentLength = int64(len(data))
	return req, tracked
}

func TestRetry_BodyHandling(t *testing.T) {
	data := []byte(`{"payload":"` + strings.Repeat("x", 4096) + `"}`)

	t.Run("body without GetBody is not retried", func(t *testing.T) {
		client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, http.StatusServiceUnavailable, "busy")
		})
		req, err := client.NewRequest(context.Background(), http.MethodGet, retryTestPath, io.MultiReader(bytes.NewReader(data)))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Do(req, nil)
		if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != http.StatusServiceUnavailable { //nolint:errorlint // raw error expected
			t.Errorf("expected the original 503 error, got %T: %v", err, err)
		}
		if hits.Load() != 1 {
			t.Errorf("expected 1 attempt, got %d", hits.Load())
		}
	})

	t.Run("GetBody failure is joined with the last error", func(t *testing.T) {
		client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, http.StatusServiceUnavailable, "busy")
		})
		req, _ := newTrackedRequest(t, client, http.MethodGet, data)
		getBodyErr := errors.New("cannot reopen")
		req.GetBody = func() (io.ReadCloser, error) { return nil, getBodyErr }
		_, err := client.Do(req, nil)
		var apiErr *APIError
		if !errors.Is(err, getBodyErr) || !errors.As(err, &apiErr) {
			t.Errorf("expected both GetBody error and API error, got %v", err)
		}
		if hits.Load() != 1 {
			t.Errorf("expected 1 attempt, got %d", hits.Load())
		}
	})

	t.Run("replay body is closed when canceled during backoff", func(t *testing.T) {
		client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, http.StatusServiceUnavailable, "busy")
		})
		client.AddRequestMiddleware(ExponentialBackoffRetryMiddleware(3, 10*time.Second, &NoOpLogger{}))
		req, tracked := newTrackedRequest(t, client, http.MethodGet, data)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err := client.Do(req.WithContext(ctx), nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected deadline, got %v", err)
		}
		tracked.assertAllClosedOnce(t, 0) // GetBody runs only after the wait
	})

	t.Run("replay body is closed when downstream rejects the retry", func(t *testing.T) {
		client, _ := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, http.StatusTooManyRequests, "slow down")
		})
		var calls atomic.Int32
		stop := errors.New("rejected downstream")
		client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
			return func(ctx *RequestContext) error {
				if calls.Add(1) == 2 {
					return stop
				}
				return next(ctx)
			}
		})
		req, tracked := newTrackedRequest(t, client, http.MethodPost, data)
		_, err := client.Do(req, nil)
		if !errors.Is(err, stop) {
			t.Errorf("expected the downstream error, got %v", err)
		}
		tracked.assertAllClosedOnce(t, 1)
	})

	t.Run("debug logging downstream of retry sees the full body on every attempt", func(t *testing.T) {
		var mu sync.Mutex
		var received []string
		client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body) //nolint:errcheck // compared below
			mu.Lock()
			received = append(received, string(b))
			mu.Unlock()
			if hit == 1 {
				writeAPIError(w, http.StatusTooManyRequests, "slow down")
				return
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		})
		logger := &syncLogger{}
		client.AddRequestMiddleware(DebugLoggingMiddleware(logger))
		req, tracked := newTrackedRequest(t, client, http.MethodPost, data)
		if _, err := client.Do(req, nil); err != nil {
			t.Fatal(err)
		}
		if hits.Load() != 2 || len(received) != 2 || received[0] != string(data) || received[1] != string(data) {
			t.Errorf("hits=%d, bodies received: %d (identical: %v)", hits.Load(), len(received), len(received) == 2 && received[0] == received[1])
		}
		if n := strings.Count(logger.text(), "Request Body (truncated)"); n != 2 {
			t.Errorf("expected both attempts to log the body, got %d logs", n)
		}
		tracked.assertAllClosedOnce(t, 1)
	})
}

func TestRetry_MethodActuallySentDecides(t *testing.T) {
	client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusServiceUnavailable, "busy")
	})
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Request.Method = http.MethodPost // rewritten after the retry middleware decided nothing yet
			return next(ctx)
		}
	})
	_, _ = doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil) //nolint:errcheck // asserted via hits
	if hits.Load() != 1 {
		t.Errorf("a POST must not be retried on 503, got %d attempts", hits.Load())
	}
}

// largeSlowBody is a body whose reads are delayed, so the request is still being
// written when the server answers.
type largeSlowBody struct {
	remaining int
	delay     time.Duration
}

func (b *largeSlowBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}
	time.Sleep(b.delay)
	n := min(len(p), b.remaining, 32*1024)
	for i := range n {
		p[i] = 'a'
	}
	b.remaining -= n
	return n, nil
}

func TestRetry_EarlyAnswerBeforeBodyIsRead(t *testing.T) {
	const size = 1 << 20
	for _, tc := range []struct {
		method string
		status int
	}{{http.MethodGet, http.StatusServiceUnavailable}, {http.MethodPost, http.StatusTooManyRequests}} {
		t.Run(tc.method, func(t *testing.T) {
			var secondLen atomic.Int64
			client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, r *http.Request) {
				if hit == 1 {
					writeAPIError(w, tc.status, "early") // answers without reading the body
					return
				}
				n, _ := io.Copy(io.Discard, r.Body) //nolint:errcheck // length compared below
				secondLen.Store(n)
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{}`)
			})
			req, err := client.NewRequest(context.Background(), tc.method, retryTestPath, &largeSlowBody{remaining: size, delay: 2 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			req.ContentLength = size
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(&largeSlowBody{remaining: size}), nil
			}
			if _, err := client.Do(req, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hits.Load() != 2 || secondLen.Load() != size {
				t.Errorf("hits=%d, replayed %d bytes", hits.Load(), secondLen.Load())
			}
		})
	}
}

func TestRetry_WritesAreNotRetriedOnServerErrors(t *testing.T) {
	for _, tc := range []struct {
		method string
		status int
	}{
		{http.MethodPost, http.StatusBadGateway},
		{http.MethodPut, http.StatusInternalServerError},
		{http.MethodPatch, http.StatusServiceUnavailable},
		{http.MethodDelete, http.StatusServiceUnavailable},
		{http.MethodPost, http.StatusRequestTimeout},
	} {
		t.Run(fmt.Sprintf("%s %d", tc.method, tc.status), func(t *testing.T) {
			client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
				writeAPIError(w, tc.status, "failed")
			})
			_, err := doRetryRequest(t, client, context.Background(), tc.method, []byte(`{}`), nil)
			apiErr, ok := err.(*APIError) //nolint:errorlint // raw error expected
			if !ok || apiErr.StatusCode != tc.status {
				t.Fatalf("expected raw *APIError %d, got %T: %v", tc.status, err, err)
			}
			if hits.Load() != 1 {
				t.Errorf("expected exactly 1 attempt, got %d", hits.Load())
			}
		})
	}
}

func TestRetry_WriteRetriedOn429(t *testing.T) {
	data := []byte(strings.Repeat("0123456789abcdef", 64*1024)) // 1 MiB

	t.Run("original exchange, identical large body", func(t *testing.T) {
		var mu sync.Mutex
		var sums [][32]byte
		client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body) //nolint:errcheck // hash compared below
			mu.Lock()
			sums = append(sums, sha256.Sum256(b))
			mu.Unlock()
			if hit == 1 {
				writeAPIError(w, http.StatusTooManyRequests, "slow down")
				return
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		})
		if _, err := doRetryRequest(t, client, context.Background(), http.MethodPost, data, nil); err != nil {
			t.Fatal(err)
		}
		if hits.Load() != 2 || len(sums) != 2 || sums[0] != sums[1] || sums[0] != sha256.Sum256(data) {
			t.Errorf("hits=%d, identical bodies=%v", hits.Load(), len(sums) == 2 && sums[0] == sums[1])
		}
	})

	t.Run("429 with a truncated body is still retried", func(t *testing.T) {
		client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, _ *http.Request) {
			if hit == 1 {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"mess`)
				w.(http.Flusher).Flush()
				abortConnection()
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		}, WithHTTPClient(noKeepAliveClient()))
		if _, err := doRetryRequest(t, client, context.Background(), http.MethodPost, []byte(`{}`), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hits.Load() != 2 {
			t.Errorf("expected 2 attempts, got %d", hits.Load())
		}
	})
}

func dialError() error {
	return &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
}

func TestRetry_WriteTransportErrors(t *testing.T) {
	t.Run("dial error before anything is sent is retried", func(t *testing.T) {
		var calls atomic.Int32
		client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		}, WithHTTPClient(&http.Client{Transport: failingRT(func(r *http.Request) (*http.Response, error) {
			if calls.Add(1) == 1 {
				return nil, dialError()
			}
			return http.DefaultTransport.RoundTrip(r)
		})}))
		if _, err := doRetryRequest(t, client, context.Background(), http.MethodPost, []byte(`{}`), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls.Load() != 2 || hits.Load() != 1 {
			t.Errorf("transport calls=%d, server hits=%d", calls.Load(), hits.Load())
		}
	})

	t.Run("dial timeout before anything is sent is retried", func(t *testing.T) {
		var calls atomic.Int32
		client, _ := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		}, WithHTTPClient(&http.Client{Transport: failingRT(func(r *http.Request) (*http.Response, error) {
			if calls.Add(1) == 1 {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}
			}
			return http.DefaultTransport.RoundTrip(r)
		})}))
		if _, err := doRetryRequest(t, client, context.Background(), http.MethodPut, []byte(`{}`), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls.Load() != 2 {
			t.Errorf("expected 2 attempts, got %d", calls.Load())
		}
	})

	t.Run("failure after the request was written is not retried", func(t *testing.T) {
		var calls atomic.Int32
		client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		}, WithHTTPClient(&http.Client{Transport: failingRT(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			resp, err := http.DefaultTransport.RoundTrip(r) // really sends and writes the request
			if err != nil {
				return nil, err
			}
			resp.Body.Close()       //nolint:errcheck,gosec // response is discarded on purpose
			return nil, dialError() // even an error that looks like a dial failure
		})}))
		_, err := doRetryRequest(t, client, context.Background(), http.MethodPost, []byte(`{}`), nil)
		if err == nil {
			t.Fatal("expected an error")
		}
		if calls.Load() != 1 || hits.Load() != 1 {
			t.Errorf("POST must be sent once: transport calls=%d, server hits=%d", calls.Load(), hits.Load())
		}
	})

	t.Run("non-dial error is not retried", func(t *testing.T) {
		var calls atomic.Int32
		client, _ := newRetryClient(t, 3, func(int, http.ResponseWriter, *http.Request) {},
			WithHTTPClient(&http.Client{Transport: failingRT(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return nil, errors.New("connection reset by peer")
			})}))
		_, _ = doRetryRequest(t, client, context.Background(), http.MethodDelete, nil, nil) //nolint:errcheck // asserted via calls
		if calls.Load() != 1 {
			t.Errorf("expected 1 attempt, got %d", calls.Load())
		}
	})
}

// deadServerURL returns a URL nothing listens on.
func deadServerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	return url
}

func TestRetry_RedirectsBlockWriteRetries(t *testing.T) {
	post := func(t *testing.T, c *Client) error {
		_, err := doRetryRequest(t, c, context.Background(), http.MethodPost, []byte(`{"a":1}`), nil)
		return err
	}

	t.Run("303 to a dead host: the POST happens once", func(t *testing.T) {
		dead := deadServerURL(t)
		client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, dead+"/elsewhere", http.StatusSeeOther)
		})
		if err := post(t, client); err == nil {
			t.Fatal("expected an error")
		}
		if hits.Load() != 1 {
			t.Errorf("expected the POST once, got %d", hits.Load())
		}
	})

	t.Run("303 to a URL answering 429: the POST happens once", func(t *testing.T) {
		var otherHits atomic.Int32
		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			otherHits.Add(1)
			writeAPIError(w, http.StatusTooManyRequests, "slow down")
		}))
		defer other.Close()
		client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+"/x", http.StatusSeeOther)
		})
		if err := post(t, client); err == nil {
			t.Fatal("expected an error")
		}
		if hits.Load() != 1 || otherHits.Load() != 1 {
			t.Errorf("POST hits=%d, redirect target hits=%d", hits.Load(), otherHits.Load())
		}
	})

	t.Run("307 A to B and back to A, then 429: no retry", func(t *testing.T) {
		var serverURL string
		var bHits atomic.Int32
		b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bHits.Add(1)
			http.Redirect(w, r, serverURL+retryTestPath, http.StatusTemporaryRedirect)
		}))
		defer b.Close()
		client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, r *http.Request) {
			if hit == 1 {
				http.Redirect(w, r, b.URL+"/x", http.StatusTemporaryRedirect)
				return
			}
			writeAPIError(w, http.StatusTooManyRequests, "slow down")
		})
		serverURL = client.BaseURL
		err := post(t, client)
		if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != http.StatusTooManyRequests { //nolint:errorlint // raw error expected
			t.Fatalf("expected the 429, got %T: %v", err, err)
		}
		if hits.Load() != 2 || bHits.Load() != 1 {
			t.Errorf("A hits=%d (want 2: redirect, then 429), B hits=%d", hits.Load(), bHits.Load())
		}
	})

	t.Run("same-URL 307 then 429: no retry", func(t *testing.T) {
		client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, r *http.Request) {
			if hit == 1 {
				http.Redirect(w, r, retryTestPath, http.StatusTemporaryRedirect)
				return
			}
			writeAPIError(w, http.StatusTooManyRequests, "slow down")
		})
		if err := post(t, client); err == nil {
			t.Fatal("expected an error")
		}
		if hits.Load() != 2 {
			t.Errorf("expected 2 requests in one attempt, got %d", hits.Load())
		}
	})

	redirectingRT := func(calls *atomic.Int32, second func(*http.Request) (*http.Response, error)) http.RoundTripper {
		return failingRT(func(r *http.Request) (*http.Response, error) {
			if calls.Add(1) == 1 {
				// No Response.Request and no trace callbacks: a transport that gives no provenance
				return &http.Response{
					StatusCode: http.StatusSeeOther, Status: "303 See Other",
					Header: http.Header{"Location": {retryTestPath}}, Body: http.NoBody,
				}, nil
			}
			return second(r)
		})
	}

	t.Run("custom transport: 303 then 429 without provenance: once", func(t *testing.T) {
		var calls atomic.Int32
		client, _ := newRetryClient(t, 3, func(int, http.ResponseWriter, *http.Request) {},
			WithHTTPClient(&http.Client{Transport: redirectingRT(&calls, func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests",
					Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"message":"slow"}`)),
				}, nil
			})}))
		if err := post(t, client); err == nil {
			t.Fatal("expected an error")
		}
		if calls.Load() != 2 {
			t.Errorf("expected 2 transport calls (303, 429) in one attempt, got %d", calls.Load())
		}
	})

	t.Run("custom transport: 429 without provenance and no redirect is not retried", func(t *testing.T) {
		var calls atomic.Int32
		client, _ := newRetryClient(t, 3, func(int, http.ResponseWriter, *http.Request) {},
			WithHTTPClient(&http.Client{Transport: failingRT(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{
					StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests",
					Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`)),
				}, nil
			})}))
		if err := post(t, client); err == nil {
			t.Fatal("expected an error")
		}
		if calls.Load() != 1 {
			t.Errorf("fail closed: expected 1 call, got %d", calls.Load())
		}
	})

	t.Run("trace-silent transport: 303 then dial failure on the redirect hop: once", func(t *testing.T) {
		var calls atomic.Int32
		client, _ := newRetryClient(t, 3, func(int, http.ResponseWriter, *http.Request) {},
			WithHTTPClient(&http.Client{Transport: redirectingRT(&calls, func(*http.Request) (*http.Response, error) {
				return nil, dialError()
			})}))
		if err := post(t, client); err == nil {
			t.Fatal("expected an error")
		}
		if calls.Load() != 2 {
			t.Errorf("expected the POST once and its redirect once, got %d calls", calls.Load())
		}
	})
}

func TestRetry_RedirectPolicyIsPreserved(t *testing.T) {
	t.Run("caller CheckRedirect with ErrUseLastResponse", func(t *testing.T) {
		var checked atomic.Int32
		hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			checked.Add(1)
			return http.ErrUseLastResponse
		}}
		client, hits := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		}, WithHTTPClient(hc))
		_, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
		if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != http.StatusFound { //nolint:errorlint // raw error expected
			t.Errorf("expected the 302 as the result, got %T: %v", err, err)
		}
		if checked.Load() != 1 || hits.Load() != 1 {
			t.Errorf("CheckRedirect calls=%d, hits=%d", checked.Load(), hits.Load())
		}
	})

	t.Run("default policy stops after 10 redirects with Go's error", func(t *testing.T) {
		client, hits := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, retryTestPath, http.StatusFound)
		})
		_, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
			t.Errorf("expected Go's redirect error, got %v", err)
		}
		if hits.Load() != 10 { // Go's default: the original request plus 9 redirects
			t.Errorf("expected 10 requests, got %d", hits.Load())
		}
	})

	t.Run("the shared http.Client is not modified", func(t *testing.T) {
		custom := func(*http.Request, []*http.Request) error { return nil }
		hc := &http.Client{CheckRedirect: custom}
		client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{}`)
		}, WithHTTPClient(hc))
		before := reflect.ValueOf(hc.CheckRedirect).Pointer()

		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 5 {
					if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err != nil {
						t.Error(err)
					}
				}
			})
		}
		wg.Wait()
		if after := reflect.ValueOf(client.HTTPClient.CheckRedirect).Pointer(); after != before {
			t.Error("CheckRedirect of the shared client changed")
		}

		plain := &http.Client{}
		client2, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) }, WithHTTPClient(plain))
		if _, err := doRetryRequest(t, client2, context.Background(), http.MethodGet, nil, nil); err != nil {
			t.Fatal(err)
		}
		if plain.CheckRedirect != nil {
			t.Error("a nil CheckRedirect was replaced on the shared client")
		}
	})
}

func TestRetry_CallerTraceStillFires(t *testing.T) {
	var gotConn, wrote atomic.Int32
	trace := &httptrace.ClientTrace{
		GotConn:      func(httptrace.GotConnInfo) { gotConn.Add(1) },
		WroteRequest: func(httptrace.WroteRequestInfo) { wrote.Add(1) },
	}
	client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) })
	ctx := httptrace.WithClientTrace(context.Background(), trace)
	if _, err := doRetryRequest(t, client, ctx, http.MethodGet, nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotConn.Load() == 0 || wrote.Load() == 0 {
		t.Errorf("caller trace callbacks did not fire: GotConn=%d WroteRequest=%d", gotConn.Load(), wrote.Load())
	}
}

func TestRetry_SensitivePathsDoNotLogErrorText(t *testing.T) {
	const (
		msgSecret    = "SECRET-IN-MESSAGE"
		detailSecret = "SECRET-IN-DETAILS"
		rawSecret    = "SECRET-IN-RAW-BODY"
	)
	bodies := map[string]string{
		"message":      fmt.Sprintf(`{"code":"err","message":%q}`, msgSecret),
		"details":      fmt.Sprintf(`{"code":"err","message":"failed","details":%q}`, detailSecret),
		"unrecognized": rawSecret,
	}
	markers := map[string]string{"message": msgSecret, "details": detailSecret, "unrecognized": rawSecret}

	run := func(path, kind, method string) string {
		logger := &syncLogger{}
		client, _ := newRetryClientAt(t, path, 2, func(_ int, w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, bodies[kind])
		}, WithLogger(logger))
		AddDetailedDebugLogging(client)
		req, err := client.NewRequest(context.Background(), method, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Do(req, nil); err == nil {
			t.Fatal("expected an error")
		}
		return logger.text()
	}

	for kind, marker := range markers {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(kind+" "+method, func(t *testing.T) {
				logs := run("/clusters/c1/kubeconfigs", kind, method)
				if strings.Contains(logs, marker) {
					t.Errorf("secret leaked into logs:\n%s", logs)
				}
				if !strings.Contains(logs, "/kubeconfigs") || !strings.Contains(logs, "500") {
					t.Errorf("expected method, path and status to stay in the logs:\n%s", logs)
				}
			})
		}
		t.Run(kind+" control on a normal path", func(t *testing.T) {
			if logs := run(retryTestPath, kind, http.MethodGet); !strings.Contains(logs, marker) {
				t.Errorf("non-sensitive paths should keep today's log content:\n%s", logs)
			}
		})
	}
}
