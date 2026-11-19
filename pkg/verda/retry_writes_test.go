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
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

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

func TestRetry_RedirectPolicyFailureIsNotRetried(t *testing.T) {
	t.Run("default 10-hop limit", func(t *testing.T) {
		client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, retryTestPath, http.StatusFound)
		})
		_, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
			t.Errorf("expected Go's redirect error, got %v", err)
		}
		if hits.Load() != 10 {
			t.Errorf("expected one attempt of 10 requests, got %d", hits.Load())
		}
	})

	t.Run("error returned by the caller's CheckRedirect", func(t *testing.T) {
		refused := errors.New("redirects are not allowed")
		hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return refused }}
		client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, retryTestPath, http.StatusFound)
		}, WithHTTPClient(hc))
		_, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil)
		if !errors.Is(err, refused) {
			t.Errorf("expected the policy error, got %v", err)
		}
		if hits.Load() != 1 {
			t.Errorf("expected 1 request, got %d", hits.Load())
		}
	})

	t.Run("a dial error on a redirect hop is still retried for reads", func(t *testing.T) {
		dead := deadServerURL(t)
		client, hits := newRetryClient(t, 2, func(_ int, w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, dead+"/x", http.StatusFound)
		})
		_, _ = doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil) //nolint:errcheck // asserted via hits
		if hits.Load() != 3 {
			t.Errorf("expected 3 attempts, got %d", hits.Load())
		}
	})
}

func TestNeverReachedServer(t *testing.T) {
	const orig = "http://api.test/retry-test"
	dial := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")}
	wrap := func(rawURL string, err error) error { return &url.Error{Op: "Post", URL: rawURL, Err: err} }

	tests := []struct {
		name string
		te   *transportError
		want bool
	}{
		{"dial error on the original URL", &transportError{err: wrap(orig, dial), origURL: orig}, true},
		{"DNS error on the original URL", &transportError{err: wrap(orig, &net.DNSError{Err: "no such host", Name: "api.test"}), origURL: orig}, true},
		{"after a redirect", &transportError{err: wrap(orig, dial), origURL: orig, redirected: true}, false},
		{"after a write", &transportError{err: wrap(orig, dial), origURL: orig, writes: 1}, false},
		{"failure about another URL", &transportError{err: wrap("http://other.test/x", dial), origURL: orig}, false},
		{"not a dial or DNS error", &transportError{err: wrap(orig, errors.New("connection reset")), origURL: orig}, false},
		{"no url.Error around it", &transportError{err: dial, origURL: orig}, false},
		{"proxy connection failure", &transportError{err: wrap(orig, &net.OpError{Op: "proxyconnect", Err: errors.New("refused")}), origURL: orig}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := neverReachedServer(tt.te); got != tt.want {
				t.Errorf("neverReachedServer() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Each condition of the write-429 provenance rule is pinned on its own: a
// transport that forges provenance on one of the three still must not get a retry.
func TestRetry_WriteTooManyRequestsProvenance(t *testing.T) {
	tooMany := func(request *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests",
			Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"message":"slow"}`)), Request: request,
		}
	}
	created := &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}

	run := func(t *testing.T, rt func(call int, r *http.Request) *http.Response) int32 {
		var calls atomic.Int32
		client, _ := newRetryClient(t, 3, func(int, http.ResponseWriter, *http.Request) {},
			WithHTTPClient(&http.Client{Transport: failingRT(func(r *http.Request) (*http.Response, error) {
				return rt(int(calls.Add(1)), r), nil
			})}))
		_, _ = doRetryRequest(t, client, context.Background(), http.MethodPost, []byte(`{}`), nil) //nolint:errcheck // asserted via calls
		return calls.Load()
	}

	t.Run("original exchange is retried", func(t *testing.T) {
		got := run(t, func(call int, r *http.Request) *http.Response {
			if call == 1 {
				return tooMany(&http.Request{})
			}
			return created
		})
		if got != 2 {
			t.Errorf("expected a retry, got %d calls", got)
		}
	})

	t.Run("no redirect, but the response says it answers a redirected request", func(t *testing.T) {
		got := run(t, func(int, *http.Request) *http.Response {
			return tooMany(&http.Request{Response: &http.Response{}})
		})
		if got != 1 {
			t.Errorf("expected no retry, got %d calls", got)
		}
	})

	t.Run("a redirect happened, but the response claims to answer the original request", func(t *testing.T) {
		got := run(t, func(call int, r *http.Request) *http.Response {
			if call == 1 {
				return &http.Response{
					StatusCode: http.StatusSeeOther, Status: "303 See Other",
					Header: http.Header{"Location": {retryTestPath}}, Body: http.NoBody,
				}
			}
			return tooMany(&http.Request{})
		})
		if got != 2 {
			t.Errorf("expected one attempt (redirect plus 429) and no retry, got %d calls", got)
		}
	})
}

func TestRetry_WrappedErrUseLastResponseIsAPolicyFailure(t *testing.T) {
	// net/http compares with ==, so a wrapped sentinel fails the request
	hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return fmt.Errorf("policy: %w", http.ErrUseLastResponse)
	}}
	client, hits := newRetryClient(t, 3, func(_ int, w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, retryTestPath, http.StatusFound)
	}, WithHTTPClient(hc))
	if _, err := doRetryRequest(t, client, context.Background(), http.MethodGet, nil, nil); err == nil {
		t.Fatal("expected an error")
	}
	if hits.Load() != 1 {
		t.Errorf("a policy failure must not be retried, got %d requests", hits.Load())
	}
}
