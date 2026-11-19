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
	"time"
)

type RequestContext struct {
	Method  string
	Path    string
	Body    any
	Headers http.Header
	Query   url.Values
	Request *http.Request
	Client  *Client
}

// Context returns the context of the underlying request, or context.Background() if there is none
func (ctx *RequestContext) Context() context.Context {
	if ctx.Request == nil {
		return context.Background()
	}
	return ctx.Request.Context()
}

type ResponseContext struct {
	Request    *RequestContext
	Response   *http.Response
	Body       []byte
	Error      error
	StatusCode int
}

type RequestMiddleware func(next RequestHandler) RequestHandler

type ResponseMiddleware func(next ResponseHandler) ResponseHandler

type RequestHandler func(ctx *RequestContext) error

type ResponseHandler func(ctx *ResponseContext) error

func AuthenticationMiddleware() RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			bearerToken, err := ctx.Client.Auth.GetBearerTokenContext(ctx.Context())
			if err != nil {
				return fmt.Errorf("failed to get authentication token: %w", err)
			}

			if bearerToken == "" {
				return fmt.Errorf("empty authentication token")
			}

			ctx.Headers.Set("Authorization", bearerToken)

			return next(ctx)
		}
	}
}

func ContentTypeMiddleware(contentType string) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			if ctx.Body != nil {
				ctx.Headers.Set("Content-Type", contentType)
			}
			return next(ctx)
		}
	}
}

func JSONContentTypeMiddleware() RequestMiddleware {
	return ContentTypeMiddleware("application/json")
}

// cryptoRandFloat64 uses crypto/rand for jitter to prevent thundering herd on retries
func cryptoRandFloat64() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	// Convert random bytes to float64 in [0,1) using mantissa bits
	return float64(binary.BigEndian.Uint64(b[:])&((1<<53)-1)) / (1 << 53)
}

func LoggingMiddleware(logger Logger) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			start := time.Now()
			logger.Debug("Starting %s request to %s", ctx.Method, ctx.Path)

			err := next(ctx)

			duration := time.Since(start)
			if err != nil {
				logger.Debug("Request %s %s failed after %v: %s", ctx.Method, ctx.Path, duration, loggableError(isSensitiveRequest(ctx), err))
			} else {
				logger.Debug("Request %s %s completed in %v", ctx.Method, ctx.Path, duration)
			}

			return err
		}
	}
}

func UserAgentMiddleware(userAgent string) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Headers.Set("User-Agent", userAgent)
			return next(ctx)
		}
	}
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
// *bytes.Buffer, *bytes.Reader and *strings.Reader bodies. A middleware that transforms the
// body must update GetBody and ContentLength, or retries resend the original
// bytes. Code that a middleware placed before this one runs after next() now
// runs after the response comes back, not before the request is sent.
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
// about the original URL (not a redirect hop).
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

// ErrorHandlingMiddleware sets ctx.Error to an *APIError for non-2xx responses.
// An *APIError already set by Client.Do is kept, so each body is decoded once.
func ErrorHandlingMiddleware() ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			var apiErr *APIError
			if (ctx.StatusCode < 200 || ctx.StatusCode >= 300) && !errors.As(ctx.Error, &apiErr) {
				ctx.Error = parseAPIError(ctx.StatusCode, ctx.Body)
			}

			return next(ctx)
		}
	}
}

func ResponseLoggingMiddleware(logger Logger) ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			logger.Debug("Response for %s %s: Status %d, Body length: %d bytes",
				ctx.Request.Method, ctx.Request.Path, ctx.StatusCode, len(ctx.Body))

			if ctx.Error != nil {
				logger.Debug("Response error: %s", loggableError(ctx.Request != nil && isSensitiveRequest(ctx.Request), ctx.Error))
			}

			return next(ctx)
		}
	}
}

// MetricsMiddleware is a placeholder for collecting request/response metrics
func MetricsMiddleware(logger Logger) ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			logger.Debug("Metrics: %s %s -> %d (%d bytes)",
				ctx.Request.Method, ctx.Request.Path, ctx.StatusCode, len(ctx.Body))

			return next(ctx)
		}
	}
}

// CacheMiddleware is a placeholder for response caching
func CacheMiddleware() ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			return next(ctx)
		}
	}
}

// DebugLoggingMiddleware logs full request details - redacts auth headers and skips token endpoints
func DebugLoggingMiddleware(logger Logger) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			if isSensitiveRequest(ctx) {
				return next(ctx)
			}

			logger.Info("=== API Request ===")
			logger.Info("Method: %s", ctx.Method)
			logger.Info("Path: %s", ctx.Path)

			if len(ctx.Query) > 0 {
				logger.Info("Query Parameters: %s", ctx.Query.Encode())
			}

			logger.Info("Headers:")
			for name, values := range ctx.Headers {
				if name == "Authorization" {
					logger.Info("  %s: [REDACTED]", name)
				} else {
					for _, value := range values {
						logger.Info("  %s: %s", name, value)
					}
				}
			}

			// Log request body if present
			if ctx.Request != nil && ctx.Request.Body != nil {
				// Read the body
				bodyBytes, err := io.ReadAll(ctx.Request.Body)
				if err != nil {
					logger.Info("Request Body: [error reading body: %v]", err)
				} else {
					// Restore the body for the actual request
					ctx.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

					if len(bodyBytes) > 0 {
						bodyStr := string(bodyBytes)
						if len(bodyStr) > 1000 {
							logger.Info("Request Body (truncated): %s...", bodyStr[:1000])
						} else {
							logger.Info("Request Body: %s", bodyStr)
						}
					} else {
						logger.Info("Request Body: [empty]")
					}
				}
			} else {
				logger.Info("Request Body: [none]")
			}

			return next(ctx)
		}
	}
}

// sensitivePaths are path fragments whose requests and responses the debug
// logging middleware skips: token requests carry credentials, and
// kubeconfig responses carry client private keys.
var sensitivePaths = []string{"/token", "/kubeconfigs"}

func isSensitivePath(path string) bool {
	for _, p := range sensitivePaths {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}

// isSensitiveRequest checks both the path the middleware chain saw and the path of the request itself.
func isSensitiveRequest(ctx *RequestContext) bool {
	if ctx == nil {
		return false
	}
	if isSensitivePath(ctx.Path) {
		return true
	}
	return ctx.Request != nil && ctx.Request.URL != nil && isSensitivePath(ctx.Request.URL.Path)
}

// loggableError renders err for a log line. On a sensitive path the error text
// may echo the response body (parseAPIError falls back to the raw body), so only
// the status is kept.
func loggableError(sensitive bool, err error) string {
	if !sensitive {
		return err.Error()
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("status %d (details omitted)", apiErr.StatusCode)
	}
	return "details omitted"
}

func DebugResponseLoggingMiddleware(logger Logger) ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			if ctx.Request != nil && isSensitiveRequest(ctx.Request) {
				return next(ctx)
			}

			logger.Info("=== API Response ===")
			logger.Info("Status Code: %d", ctx.StatusCode)

			if ctx.Response != nil {
				logger.Info("Response Headers:")
				for name, values := range ctx.Response.Header {
					for _, value := range values {
						logger.Info("  %s: %s", name, value)
					}
				}
			}

			if len(ctx.Body) > 0 {
				bodyStr := string(ctx.Body)
				if len(bodyStr) > 1000 {
					logger.Info("Response Body (truncated): %s...", bodyStr[:1000])
				} else {
					logger.Info("Response Body: %s", bodyStr)
				}
			} else {
				logger.Info("Response Body: [empty]")
			}

			if ctx.Error != nil {
				logger.Info("Error: %v", ctx.Error)
			}
			logger.Info("==================")

			return next(ctx)
		}
	}
}

// AddDetailedDebugLogging adds verbose request/response debug logging to the client.
// Call this after creating your client if you want to see full HTTP requests/responses.
func AddDetailedDebugLogging(client *Client) {
	client.AddRequestMiddleware(DebugLoggingMiddleware(client.Logger))
	client.AddResponseMiddleware(DebugResponseLoggingMiddleware(client.Logger))
}
