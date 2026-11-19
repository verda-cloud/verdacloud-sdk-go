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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func TestDo_HeadersSetDirectlyOnTheRequestAreSent(t *testing.T) {
	var mu sync.Mutex
	var seen []http.Header
	client, hits := newRetryClient(t, 2, func(hit int, w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		if hit == 1 {
			writeAPIError(w, http.StatusServiceUnavailable, "busy")
			return
		}
		fmt.Fprint(w, `{}`)
	})
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Request.Header.Set("X-Signature", "abc") // directly on the request, not on ctx.Headers
			ctx.Headers.Set("X-Context", "ctx")
			return next(ctx)
		}
	})
	if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", hits.Load())
	}
	for i, h := range seen {
		if h.Get("X-Signature") != "abc" || h.Get("X-Context") != "ctx" {
			t.Errorf("attempt %d lost a header: %v", i+1, h)
		}
	}
}

func TestDo_HeaderOfTheCallersRequestDeletedByMiddlewareIsNotSent(t *testing.T) {
	var got http.Header
	client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		fmt.Fprint(w, `{}`)
	})
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Headers.Del("X-Caller")
			return next(ctx)
		}
	})
	req, err := client.NewRequest(context.Background(), http.MethodGet, retryTestPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Caller", "set by the caller")
	req.Header.Set("X-Kept", "kept")
	if _, err := client.Do(req, nil); err != nil {
		t.Fatal(err)
	}
	if got.Get("X-Caller") != "" || got.Get("X-Kept") != "kept" {
		t.Errorf("unexpected headers: %v", got)
	}
}

func TestRetry_MethodSentIsClassifiedEvenIfRestoredAfterNext(t *testing.T) {
	client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusServiceUnavailable, "busy")
	})
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Request.Method = http.MethodPost
			defer func() { ctx.Request.Method = http.MethodGet }() // restored before the retry middleware looks
			return next(ctx)
		}
	})
	_, _ = doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil) //nolint:errcheck // asserted via hits
	if hits.Load() != 1 {
		t.Errorf("the POST that was sent must not be retried on 503, got %d attempts", hits.Load())
	}
}

func TestRetry_StackedRetryMiddlewaresDoNotMultiplyAttempts(t *testing.T) {
	client, hits := newRetryClient(t, 2, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusServiceUnavailable, "busy")
	})
	client.AddRequestMiddleware(ExponentialBackoffRetryMiddleware(2, time.Millisecond, &NoOpLogger{}))
	resp, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
	if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != http.StatusServiceUnavailable { //nolint:errorlint // raw error expected
		t.Errorf("expected the raw 503 error, got %T: %v", err, err)
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected the last response, got %v", resp)
	}
	if hits.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", hits.Load())
	}
}

func TestRetryDelay(t *testing.T) {
	tests := []struct {
		name       string
		initial    time.Duration
		attempt    int
		jitter     float64
		retryAfter time.Duration
		want       time.Duration
	}{
		{"first retry without jitter", 10 * time.Millisecond, 1, 0, 0, 10 * time.Millisecond},
		{"doubles each retry", 10 * time.Millisecond, 3, 0, 0, 40 * time.Millisecond},
		{"minus 50% jitter", 10 * time.Millisecond, 1, -0.5, 0, 5 * time.Millisecond},
		{"plus 50% jitter", 10 * time.Millisecond, 1, 0.5, 0, 15 * time.Millisecond},
		{"backoff is capped at 30s", time.Second, 20, 0, 0, maxRetryDelay},
		{"the capped backoff keeps its jitter", time.Second, 20, -0.5, 0, maxRetryDelay / 2},
		{"jitter cannot lift a capped wait above 30s", time.Second, 20, 0.5, 0, maxRetryDelay},
		{"jitter on a wait just under the cap is also capped", 25 * time.Second, 1, 0.5, 0, maxRetryDelay},
		{"Retry-After above the backoff wins", time.Second, 1, 0, 20 * time.Second, 20 * time.Second},
		{"backoff above Retry-After wins", 25 * time.Second, 1, 0, 20 * time.Second, 25 * time.Second},
		{"Retry-After is capped at 30s", time.Second, 1, 0, time.Hour, maxRetryDelay},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryDelay(tt.initial, tt.attempt, tt.jitter, tt.retryAfter); got != tt.want {
				t.Errorf("retryDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDebugLoggersCheckTheRequestURLPathToo(t *testing.T) {
	sensitive := &http.Request{Method: http.MethodGet, URL: mustParseURL(t, "https://api.test/clusters/c1/kubeconfigs"), Header: http.Header{}}

	t.Run("request", func(t *testing.T) {
		logger := &syncLogger{}
		handler := DebugLoggingMiddleware(logger)(func(*RequestContext) error { return nil })
		// ctx.Path was rewritten to something harmless; the request still points at kubeconfigs
		if err := handler(&RequestContext{Method: http.MethodGet, Path: "/harmless", Headers: http.Header{}, Request: sensitive}); err != nil {
			t.Fatal(err)
		}
		if out := logger.text(); out != "" {
			t.Errorf("expected nothing logged, got:\n%s", out)
		}
	})

	t.Run("response", func(t *testing.T) {
		logger := &syncLogger{}
		handler := DebugResponseLoggingMiddleware(logger)(func(*ResponseContext) error { return nil })
		reqCtx := &RequestContext{Method: http.MethodGet, Path: "/harmless", Request: sensitive}
		err := handler(&ResponseContext{Request: reqCtx, StatusCode: http.StatusOK, Body: []byte("PRIVATE-KEY-MATERIAL")})
		if err != nil {
			t.Fatal(err)
		}
		if out := logger.text(); out != "" {
			t.Errorf("expected nothing logged, got:\n%s", out)
		}
	})
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestDo_MiddlewareSwappingInARequestWithoutURLDoesNotPanic(t *testing.T) {
	client, _ := newRetryClient(t, 0, func(int, http.ResponseWriter, *http.Request) {})
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Request = &http.Request{Method: http.MethodGet, Header: http.Header{}}
			return next(ctx)
		}
	})
	if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err == nil {
		t.Error("expected an error for a request without URL")
	}
}

func TestRetryMiddleware_ToleratesNilLoggerAndNegativeRetries(t *testing.T) {
	var apiErr *APIError

	// Negative retries still send once
	calls := 0
	handler := ExponentialBackoffRetryMiddleware(-1, time.Microsecond, nil)(func(*RequestContext) error {
		calls++
		return &APIError{StatusCode: http.StatusServiceUnavailable, Message: "busy"}
	})
	if err := handler(&RequestContext{Method: http.MethodGet, Path: "/x"}); !errors.As(err, &apiErr) || calls != 1 {
		t.Errorf("expected the request to be sent once and fail with the API error, got calls=%d err=%v", calls, err)
	}

	// A retry reaches logger.Debug: a nil logger must not panic there
	calls = 0
	handler = ExponentialBackoffRetryMiddleware(1, time.Microsecond, nil)(func(*RequestContext) error {
		calls++
		return &APIError{StatusCode: http.StatusServiceUnavailable, Message: "busy"}
	})
	if err := handler(&RequestContext{Method: http.MethodGet, Path: "/x"}); !errors.As(err, &apiErr) || calls != 2 {
		t.Errorf("expected 2 attempts, got calls=%d err=%v", calls, err)
	}
}

// The context check before the backoff wait is what stops a retry when the timer
// and the context are ready at the same moment: select alone would pick either.
func TestRetryMiddleware_NeverRetriesAfterCancellation(t *testing.T) {
	for i := range 200 {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		handler := ExponentialBackoffRetryMiddleware(3, time.Microsecond, &NoOpLogger{})(func(*RequestContext) error {
			calls++
			cancel()
			time.Sleep(time.Millisecond) // the 1µs backoff timer is due when the loop continues
			return &APIError{StatusCode: http.StatusServiceUnavailable, Message: "busy"}
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.test/x", nil)
		if err != nil {
			t.Fatal(err)
		}
		err = handler(&RequestContext{Method: http.MethodGet, Path: "/x", Request: req})
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("run %d: expected cancellation after 1 attempt, got calls=%d err=%v", i, calls, err)
		}
	}
}

func TestDo_MiddlewareWrappingNextErrorKeepsTheResult(t *testing.T) {
	for _, retries := range []int{0, 2} {
		t.Run(fmt.Sprintf("retries=%d", retries), func(t *testing.T) {
			client, hits := newRetryClient(t, retries, func(_ int, w http.ResponseWriter, _ *http.Request) {
				writeAPIError(w, http.StatusNotFound, "missing")
			})
			var runs atomic.Int32
			client.AddResponseMiddleware(func(next ResponseHandler) ResponseHandler {
				return func(ctx *ResponseContext) error { runs.Add(1); return next(ctx) }
			})
			// A tracing-style middleware that annotates the error next returned
			client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
				return func(ctx *RequestContext) error {
					if err := next(ctx); err != nil {
						return fmt.Errorf("tracing: %w", err)
					}
					return nil
				}
			})
			resp, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
			if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != http.StatusNotFound { //nolint:errorlint // raw error expected
				t.Errorf("expected the raw *APIError, got %T: %v", err, err)
			}
			if resp == nil || resp.StatusCode != http.StatusNotFound {
				t.Errorf("expected the response, got %v", resp)
			}
			if runs.Load() != 1 || hits.Load() != 1 {
				t.Errorf("response middleware runs=%d, requests=%d", runs.Load(), hits.Load())
			}
		})
	}
}

func TestDo_HeadersOfTheChainStayOnTheCallersRequest(t *testing.T) {
	client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) })
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Headers.Set("X-Request-Id", "abc")
			return next(ctx)
		}
	})
	req, err := client.NewRequest(context.Background(), http.MethodGet, retryTestPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(req, nil); err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("X-Request-Id"); got != "abc" {
		t.Errorf("caller's request lost the middleware header: %q", got)
	}
	if req.Header.Get("Authorization") == "" {
		t.Error("expected the Authorization header on the caller's request, as before")
	}
}

func TestRetry_StackedMiddlewaresReportCancellationOnce(t *testing.T) {
	client, _ := newRetryClient(t, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusServiceUnavailable, "busy")
	})
	client.AddRequestMiddleware(ExponentialBackoffRetryMiddleware(3, 10*time.Second, &NoOpLogger{}))
	client.AddRequestMiddleware(ExponentialBackoffRetryMiddleware(3, 10*time.Second, &NoOpLogger{}))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := doRetryRequest(t, client, ctx, http.MethodGet, nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected the deadline, got %v", err)
	}
	if n := strings.Count(err.Error(), "request canceled after"); n != 1 {
		t.Errorf("cancellation reported %d times: %v", n, err)
	}
}

func TestRetry_RedactionUsesTheRequestThatWasSent(t *testing.T) {
	const secret = "SECRET-IN-RAW-BODY"
	const sensitivePath = "/clusters/c1/kubeconfigs"

	t.Run("a middleware swaps the URL and restores it after next", func(t *testing.T) {
		logger := &syncLogger{}
		client, _ := newRetryClientAt(t, sensitivePath, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, secret)
		}, WithLogger(logger))
		client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
			return func(ctx *RequestContext) error {
				original := ctx.Request.URL
				swapped := *original
				swapped.Path = sensitivePath
				ctx.Request.URL = &swapped
				defer func() { ctx.Request.URL = original }() // the outer loggers see a harmless path again
				return next(ctx)
			}
		})
		req, err := client.NewRequest(context.Background(), http.MethodGet, "/harmless", nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Do(req, nil); err == nil {
			t.Fatal("expected an error")
		}
		if logs := logger.text(); strings.Contains(logs, secret) {
			t.Errorf("secret leaked into logs:\n%s", logs)
		}
	})

	t.Run("a redirect leads to a sensitive path", func(t *testing.T) {
		logger := &syncLogger{}
		client, _ := newRetryClientAt(t, retryTestPath, 0, func(_ int, w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, sensitivePath, http.StatusTemporaryRedirect)
		}, WithLogger(logger))
		client.ClearResponseMiddleware()
		client.AddResponseMiddleware(ErrorHandlingMiddleware())
		client.AddResponseMiddleware(ResponseLoggingMiddleware(logger))
		// The redirect target answers with the secret in an unrecognized body
		ms := testutil.NewMockServer()
		defer ms.Close()
		ms.SetHandler(http.MethodGet, sensitivePath, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, secret)
		})
		client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == sensitivePath {
				target := *r.URL
				target.Scheme, target.Host = "http", strings.TrimPrefix(ms.URL(), "http://")
				r = r.Clone(r.Context())
				r.URL = &target
			}
			return http.DefaultTransport.RoundTrip(r)
		})}
		if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err == nil {
			t.Fatal("expected an error")
		}
		if logs := logger.text(); strings.Contains(logs, secret) {
			t.Errorf("secret leaked into logs:\n%s", logs)
		}
	})
}

func TestRetry_ReplayBodyIsClosedOnceWhenTheTransportClosesIt(t *testing.T) {
	client, hits := newRetryClient(t, 3, func(hit int, w http.ResponseWriter, _ *http.Request) {
		if hit == 1 {
			writeAPIError(w, http.StatusTooManyRequests, "slow down")
			return
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{}`)
	})
	req, tracked := newTrackedRequest(t, client, http.MethodPost, []byte(`{"a":1}`))
	if _, err := client.Do(req, nil); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", hits.Load())
	}
	// The transport closes the replay body, and the middleware closes it again: still one real Close
	tracked.assertAllClosedOnce(t, 1)
}

func TestSentMethod(t *testing.T) {
	plain := errors.New("plain")
	tests := []struct {
		name string
		ctx  *RequestContext
		err  error
		want string
	}{
		{"method recorded on a transport error wins", &RequestContext{Method: "GET", Request: &http.Request{Method: "GET"}}, &transportError{method: "POST"}, "POST"},
		{"method recorded on an attempt error wins", &RequestContext{Method: "GET", Request: &http.Request{Method: "GET"}}, &attemptError{method: "PUT"}, "PUT"},
		{"plain error uses the request", &RequestContext{Method: "GET", Request: &http.Request{Method: "DELETE"}}, plain, "DELETE"},
		{"plain error without a request uses the context method", &RequestContext{Method: "PATCH"}, plain, "PATCH"},
		{"empty method is GET", &RequestContext{}, plain, "GET"},
		{"empty recorded method is GET", &RequestContext{}, &transportError{}, "GET"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sentMethod(tt.ctx, tt.err); got != tt.want {
				t.Errorf("sentMethod() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRetry_MethodSentIsClassifiedOnTransportErrors(t *testing.T) {
	var calls atomic.Int32
	client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{}`)
	}, WithHTTPClient(&http.Client{Transport: failingRT(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		resp, err := http.DefaultTransport.RoundTrip(r) // the POST really is written
		if err != nil {
			return nil, err
		}
		resp.Body.Close() //nolint:errcheck,gosec // response is discarded on purpose
		return nil, errors.New("connection reset by peer")
	})}))
	client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			defer func() { ctx.Request.Method = http.MethodGet }() // restored before the retry middleware looks
			return next(ctx)
		}
	})
	_, _ = doRetryRequest(t, client, context.Background(), http.MethodPost, []byte(`{}`), nil) //nolint:errcheck // asserted via calls
	if calls.Load() != 1 || hits.Load() != 1 {
		t.Errorf("a written POST must be sent once: transport calls=%d, server hits=%d", calls.Load(), hits.Load())
	}
}

type failingCloseBody struct {
	io.Reader
	err error
}

func (b *failingCloseBody) Close() error { return b.err }

func TestRetry_ReplayBodyCloseErrorIsRedactedOnSensitivePaths(t *testing.T) {
	const secret = "SECRET-IN-CLOSE-ERROR"
	run := func(t *testing.T, path string) string {
		logger := &syncLogger{}
		client, _ := newRetryClientAt(t, path, 0, func(_ int, w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, http.StatusTooManyRequests, "slow down")
		})
		client.AddRequestMiddleware(ExponentialBackoffRetryMiddleware(3, time.Millisecond, logger))
		var calls atomic.Int32
		client.AddRequestMiddleware(func(next RequestHandler) RequestHandler {
			return func(ctx *RequestContext) error {
				if calls.Add(1) == 2 {
					return errors.New("rejected downstream") // the replay body is never sent
				}
				return next(ctx)
			}
		})
		req, err := client.NewRequest(context.Background(), http.MethodPost, path, bytes.NewReader([]byte(`{}`)))
		if err != nil {
			t.Fatal(err)
		}
		req.GetBody = func() (io.ReadCloser, error) {
			return &failingCloseBody{Reader: strings.NewReader(`{}`), err: errors.New(secret)}, nil
		}
		_, _ = client.Do(req, nil) //nolint:errcheck // the logs are what matters
		return logger.text()
	}

	t.Run("normal path logs the close error", func(t *testing.T) {
		if logs := run(t, retryTestPath); !strings.Contains(logs, secret) {
			t.Errorf("expected the close error in the logs:\n%s", logs)
		}
	})
	t.Run("sensitive path does not", func(t *testing.T) {
		if logs := run(t, "/clusters/c1/kubeconfigs"); strings.Contains(logs, secret) {
			t.Errorf("close error leaked into logs:\n%s", logs)
		}
	})
}
