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
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// cryptoRandFloat64 uses crypto/rand for jitter to prevent thundering herd on retries
func cryptoRandFloat64() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	// Convert random bytes to float64 in [0,1) using mantissa bits
	return float64(binary.BigEndian.Uint64(b[:])&((1<<53)-1)) / (1 << 53)
}

// maxRetryDelay is the hard ceiling of a single wait between attempts.
const maxRetryDelay = 30 * time.Second

// ExponentialBackoffRetryMiddleware retries failed requests with exponential
// backoff (initialDelay * 2^n, ±50% jitter). The wait before an attempt never
// exceeds 30 seconds, and a Retry-After header on a 429 or 503 raises the wait
// up to that ceiling: the cap can shorten what the server asked for.
//
// Which requests are retried depends on the method actually sent:
//   - GET, HEAD, OPTIONS (and an empty method) are retried on 408, 429, any 5xx
//     and any transport error, including a truncated response body.
//   - POST, PUT, PATCH and DELETE are retried only when the request provably
//     never took effect: a 429 that answers the request as sent (the transport
//     must set Response.Request for this), or a dial or DNS failure before
//     anything was written. A redirect, a 5xx, a 408 or a failure after the
//     request was written is never retried, because the server may have acted.
//
// Retrying stops when the request's context is done. A request with a body is
// retried only if http.Request.GetBody is set, which NewRequest does for
// *bytes.Buffer, *bytes.Reader and *strings.Reader bodies. A middleware that
// transforms the body must update GetBody and ContentLength, or retries resend
// the original bytes. Code that a middleware placed before this one runs after next() now
// runs after the response comes back, not before the request is sent.
//
// Middleware that runs before this one, such as the default authentication
// middleware, runs once per call: every attempt sends the same bearer token. A
// middleware after this one that answers a retry without calling next ends the
// call with the previous attempt's response.
//
// This middleware does not cover calls that bypass the middleware chain, such
// as the OAuth token calls. Combine it with a retrying transport and each
// failure is retried by both.
func ExponentialBackoffRetryMiddleware(maxRetries int, initialDelay time.Duration, logger Logger) RequestMiddleware {
	const jitterPercent = 0.5 // ±50% jitter to avoid thundering herd

	maxRetries = max(maxRetries, 0)
	if logger == nil {
		logger = &NoOpLogger{}
	}

	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			var lastErr error
			callerCtx := ctx.Context()

			for attempt := 0; attempt <= maxRetries; attempt++ {
				var replay *onceCloser
				if attempt > 0 {
					// Don't retry once the caller has given up
					if callerCtx.Err() != nil {
						return fmt.Errorf("request canceled after %d attempts: %w (last error: %w)", attempt, callerCtx.Err(), lastErr)
					}
					if !canRewindBody(ctx.Request) {
						logger.Debug("Request %s %s cannot be retried: body is not replayable", ctx.Method, ctx.Path)
						return lastErr
					}

					jitter := (cryptoRandFloat64()*2 - 1) * jitterPercent
					delay := retryDelay(initialDelay, attempt, jitter, retryAfterDelay(lastErr))

					logger.Debug("Retrying request %s %s (attempt %d/%d) after %v",
						ctx.Method, ctx.Path, attempt+1, maxRetries+1, delay)
					timer := time.NewTimer(delay)
					select {
					case <-timer.C:
					case <-callerCtx.Done():
						timer.Stop()
						return fmt.Errorf("request canceled after %d attempts: %w (last error: %w)", attempt, callerCtx.Err(), lastErr)
					}

					// Open the replay body only now, so a cancel during the wait leaves nothing open
					var err error
					if replay, err = rewindBody(ctx.Request); err != nil {
						return errors.Join(lastErr, fmt.Errorf("failed to rewind request body: %w", err))
					}
				}

				lastErr = next(ctx)
				if replay != nil {
					// No-op when the transport already closed it; frees it when a middleware short-circuited
					if err := replay.Close(); err != nil {
						logger.Debug("Request %s %s: closing replay body failed: %s", ctx.Method, ctx.Path, loggableError(isSensitiveRequest(ctx), err))
					}
				}
				if lastErr == nil {
					return nil
				}

				if !retryDecision(ctx, lastErr) {
					logger.Debug("Request %s %s failed with non-retryable error: %s", ctx.Method, ctx.Path, loggableError(isSensitiveRequest(ctx), lastErr))
					return lastErr
				}
			}

			return &retryExhaustedError{last: lastErr, retries: maxRetries}
		}
	}
}

// retryDelay is the wait before retry number attempt (1 for the first retry):
// initialDelay * 2^(attempt-1) with the given jitter (a fraction, ±0.5 at most),
// raised to what the server asked for, and never above maxRetryDelay.
func retryDelay(initialDelay time.Duration, attempt int, jitter float64, retryAfter time.Duration) time.Duration {
	backoff := float64(initialDelay) * math.Pow(2, float64(attempt-1))
	backoff = math.Min(backoff, float64(maxRetryDelay))
	return min(max(time.Duration(backoff*(1+jitter)), retryAfter), maxRetryDelay)
}

// onceCloser makes Close idempotent: only the first call reaches the wrapped body.
type onceCloser struct {
	io.ReadCloser
	once sync.Once
}

func (o *onceCloser) Close() (err error) {
	o.once.Do(func() { err = o.ReadCloser.Close() })
	return err
}

// hasBody reports whether the request carries a body that a retry must resend.
func hasBody(req *http.Request) bool {
	return req != nil && req.Body != nil && req.Body != http.NoBody
}

// canRewindBody reports whether the request can be sent again.
func canRewindBody(req *http.Request) bool {
	return !hasBody(req) || req.GetBody != nil
}

// rewindBody installs a fresh copy of the request body, when there is one.
func rewindBody(req *http.Request) (*onceCloser, error) {
	if !hasBody(req) {
		return nil, nil
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	replay := &onceCloser{ReadCloser: body}
	req.Body = replay
	return replay, nil
}

// sentMethod is the method the failed attempt was sent with, which a middleware
// may have changed back after next returned. Plain errors that carry no method
// (a handler in a unit test) fall back to the request's current method. An empty
// method means GET.
func sentMethod(ctx *RequestContext, err error) string {
	var method string
	var transportErr *transportError
	var attemptErr *attemptError
	switch {
	case errors.As(err, &transportErr):
		method = transportErr.method
	case errors.As(err, &attemptErr):
		method = attemptErr.method
	case ctx.Request != nil:
		method = ctx.Request.Method
	default:
		method = ctx.Method
	}
	if method == "" {
		return http.MethodGet
	}
	return method
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// retryDecision reports whether the failed attempt may be sent again.
// Reads are retried on transient failures. Writes are retried only when the
// request provably never took effect on the server.
func retryDecision(ctx *RequestContext, err error) bool {
	var transportErr *transportError
	var attemptErr *attemptError
	var exhausted *retryExhaustedError

	// An inner retry middleware already spent its retries on this error
	if errors.As(err, &exhausted) {
		return false
	}

	if isReadMethod(sentMethod(ctx, err)) {
		if errors.As(err, &transportErr) {
			return !transportErr.redirectDenied
		}
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			return apiErr.StatusCode == http.StatusRequestTimeout ||
				apiErr.StatusCode == http.StatusTooManyRequests ||
				apiErr.StatusCode >= http.StatusInternalServerError
		}
		return shouldRetry(err)
	}

	if errors.As(err, &attemptErr) {
		return attemptErr.apiErr.StatusCode == http.StatusTooManyRequests && attemptErr.originalExchange
	}
	if errors.As(err, &transportErr) {
		return neverReachedServer(transportErr)
	}
	return false
}

// neverReachedServer reports whether a failed attempt provably sent nothing:
// no redirect, a dial or DNS failure, no request write, and the failure is
// about the original URL (not a redirect hop). It fails closed, so it is false
// for a URL with a password (url.Error strips it, so the URLs never match) and
// for a failed proxy connection (the OpError has Op "proxyconnect").
func neverReachedServer(te *transportError) bool {
	if te.redirected || te.writes != 0 {
		return false
	}
	var opErr *net.OpError
	var dnsErr *net.DNSError
	dialFailed := (errors.As(te.err, &opErr) && opErr.Op == "dial") || errors.As(te.err, &dnsErr)
	if !dialFailed {
		return false
	}
	var urlErr *url.Error
	return errors.As(te.err, &urlErr) && urlErr.URL == te.origURL
}

// retryAfterDelay is the wait the server asked for. Only 429 and 503 count.
func retryAfterDelay(err error) time.Duration {
	var attemptErr *attemptError
	if !errors.As(err, &attemptErr) {
		return 0
	}
	switch attemptErr.apiErr.StatusCode {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return parseRetryAfter(attemptErr.header.Get("Retry-After"), time.Now())
	}
	return 0
}

// parseRetryAfter reads a Retry-After value (whole seconds or an HTTP date).
// Malformed and past values give 0; anything above the retry ceiling is clamped to it.
func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if isDigits(value) {
		secs, err := strconv.ParseInt(value, 10, 64)
		if err != nil || secs >= int64(maxRetryDelay/time.Second) {
			return maxRetryDelay
		}
		return time.Duration(secs) * time.Second
	}
	date, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	return min(max(date.Sub(now), 0), maxRetryDelay)
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// retryExhaustedError is returned by the retry middleware when every attempt failed.
// Client.Do recognizes it and returns the last attempt's own error.
type retryExhaustedError struct {
	last    error
	retries int
}

func (e *retryExhaustedError) Error() string {
	return fmt.Sprintf("request failed after %d retries: %v", e.retries, e.last)
}
func (e *retryExhaustedError) Unwrap() error { return e.last }

// shouldRetry decides if we should retry based on status code - never retry auth/client errors
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		statusCode := apiErr.StatusCode

		switch statusCode {
		case http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
			http.StatusTooManyRequests,
			http.StatusRequestTimeout:
			return true
		case http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound:
			return false
		default:
			if statusCode >= 500 && statusCode < 600 {
				return true
			}
			if statusCode >= 400 && statusCode < 500 {
				return false
			}
		}
	}

	errStr := strings.ToLower(err.Error())

	nonRetryablePatterns := []string{
		"authentication",
		"unauthorized",
		"forbidden",
		"not found",
		"invalid",
		"bad request",
	}
	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	retryablePatterns := []string{
		"timeout",
		"connection",
		"temporary",
		"rate limit",
		"too many requests",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// transportError is a failed round trip. It records what the transport saw, so
// the retry middleware can tell whether the request may have reached the server.
type transportError struct {
	err        error
	writes     int32 // WroteRequest events across all hops
	redirected bool  // a redirect was followed or considered
	// redirectDenied is true when the redirect policy refused to follow a
	// redirect (Go's 10-hop limit or the caller's CheckRedirect): repeating the
	// request would hit the same policy.
	redirectDenied bool
	origURL        string // URL of the request as sent
	method         string // method of the request as sent
}

func (e *transportError) Error() string { return fmt.Sprintf("request failed: %v", e.err) }
func (e *transportError) Unwrap() error { return e.err }

// attemptError is a non-2xx response of one attempt. It wraps the *APIError
// and carries the headers and provenance the retry middleware needs.
type attemptError struct {
	apiErr *APIError
	header http.Header
	// originalExchange is true when the response answers the request as sent:
	// no redirect, and the transport tied the response to that request.
	originalExchange bool
	method           string // method of the request as sent
}

func (e *attemptError) Error() string { return e.apiErr.Error() }
func (e *attemptError) Unwrap() error { return e.apiErr }

// redirectTracker records what the redirect policy of one attempt did.
type redirectTracker struct {
	redirected atomic.Bool
	policyErr  atomic.Pointer[error]
}

// denied reports whether err is the error the redirect policy returned.
func (t *redirectTracker) denied(err error) bool {
	policyErr := t.policyErr.Load()
	return policyErr != nil && errors.Is(err, *policyErr)
}
