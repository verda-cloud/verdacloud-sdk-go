package verda

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RequestContext holds the context for request middleware
type RequestContext struct {
	Method  string
	Path    string
	Body    interface{}
	Headers http.Header
	Query   url.Values
	Request *http.Request
	Client  *Client
}

// ResponseContext holds the context for response middleware
type ResponseContext struct {
	Request    *RequestContext
	Response   *http.Response
	Body       []byte
	Error      error
	StatusCode int
}

// RequestMiddleware defines the request middleware function signature
type RequestMiddleware func(next RequestHandler) RequestHandler

// ResponseMiddleware defines the response middleware function signature
type ResponseMiddleware func(next ResponseHandler) ResponseHandler

// RequestHandler processes the request
type RequestHandler func(ctx *RequestContext) error

// ResponseHandler processes the response
type ResponseHandler func(ctx *ResponseContext) error

// AuthenticationMiddleware adds authentication headers to requests
func AuthenticationMiddleware() RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			// Get bearer token
			bearerToken, err := ctx.Client.Auth.GetBearerToken()
			if err != nil {
				return fmt.Errorf("failed to get authentication token: %w", err)
			}

			if bearerToken == "" {
				return fmt.Errorf("empty authentication token")
			}

			// Add authorization header
			ctx.Headers.Set("Authorization", bearerToken)

			return next(ctx)
		}
	}
}

// ContentTypeMiddleware sets the Content-Type header for requests with body
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

// JSONContentTypeMiddleware is a convenience middleware for JSON content type
func JSONContentTypeMiddleware() RequestMiddleware {
	return ContentTypeMiddleware("application/json")
}

// cryptoRandFloat64 generates a random float64 in [0,1) using crypto/rand
func cryptoRandFloat64() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to zero if crypto/rand fails (extremely unlikely)
		return 0
	}
	// Use the random bytes to create a float64 in [0,1)
	return float64(binary.BigEndian.Uint64(b[:])&((1<<53)-1)) / (1 << 53)
}

// LoggingMiddleware logs request details using the client's logger
func LoggingMiddleware(logger Logger) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			start := time.Now()
			logger.Debug("Starting %s request to %s", ctx.Method, ctx.Path)

			err := next(ctx)

			duration := time.Since(start)
			if err != nil {
				logger.Debug("Request %s %s failed after %v: %v", ctx.Method, ctx.Path, duration, err)
			} else {
				logger.Debug("Request %s %s completed in %v", ctx.Method, ctx.Path, duration)
			}

			return err
		}
	}
}

// UserAgentMiddleware adds a User-Agent header
func UserAgentMiddleware(userAgent string) RequestMiddleware {
	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			ctx.Headers.Set("User-Agent", userAgent)
			return next(ctx)
		}
	}
}

// ExponentialBackoffRetryMiddleware implements retry logic with exponential backoff and jitter
func ExponentialBackoffRetryMiddleware(maxRetries int, initialDelay time.Duration, logger Logger) RequestMiddleware {
	const maxDelay = 30 * time.Second
	const jitterPercent = 0.5 // 50% jitter

	return func(next RequestHandler) RequestHandler {
		return func(ctx *RequestContext) error {
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					// Calculate exponential backoff: initialDelay * 2^(attempt-1)
					baseDelay := float64(initialDelay) * math.Pow(2, float64(attempt-1))

					// Cap the delay at maxDelay
					cappedDelay := time.Duration(math.Min(baseDelay, float64(maxDelay)))

					// Add jitter: random value between -50% and +50%
					jitter := (cryptoRandFloat64()*2 - 1) * jitterPercent
					actualDelay := time.Duration(float64(cappedDelay) * (1 + jitter))

					logger.Debug("Retrying request %s %s (attempt %d/%d) after %v",
						ctx.Method, ctx.Path, attempt+1, maxRetries+1, actualDelay)
					time.Sleep(actualDelay)
				}

				lastErr = next(ctx)
				if lastErr == nil {
					return nil // Success
				}

				// Don't retry on certain errors (like authentication failures)
				if !shouldRetry(lastErr) {
					logger.Debug("Request %s %s failed with non-retryable error: %v", ctx.Method, ctx.Path, lastErr)
					break
				}
			}

			return fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
		}
	}
}

// RetryMiddleware is deprecated. Use ExponentialBackoffRetryMiddleware instead.
// Kept for backwards compatibility.
func RetryMiddleware(maxRetries int, retryDelay time.Duration, logger Logger) RequestMiddleware {
	return ExponentialBackoffRetryMiddleware(maxRetries, retryDelay, logger)
}

// shouldRetry determines if an error is retryable based on status codes and error patterns
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check if error is an APIError and inspect status code
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		statusCode := apiErr.StatusCode

		// Retry on specific server errors and rate limits
		switch statusCode {
		case http.StatusInternalServerError, // 500
			http.StatusBadGateway,         // 502
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout,     // 504
			http.StatusTooManyRequests,    // 429
			http.StatusRequestTimeout:     // 408
			return true
		case http.StatusBadRequest, // 400
			http.StatusUnauthorized, // 401
			http.StatusForbidden,    // 403
			http.StatusNotFound:     // 404
			return false
		default:
			// Retry on other 5xx errors, don't retry on other 4xx errors
			if statusCode >= 500 && statusCode < 600 {
				return true
			}
			if statusCode >= 400 && statusCode < 500 {
				return false
			}
		}
	}

	// Check error message patterns
	errStr := strings.ToLower(err.Error())

	// Non-retryable patterns
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

	// Retryable patterns
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

	// Default to non-retryable for unknown errors
	return false
}

// Response middleware implementations

// ErrorHandlingMiddleware handles HTTP error responses
func ErrorHandlingMiddleware() ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			// Check for HTTP errors
			if ctx.StatusCode < 200 || ctx.StatusCode >= 300 {
				// Try to parse as API error
				var apiError APIError
				if len(ctx.Body) > 0 {
					// This would need proper JSON parsing
					apiError = APIError{
						StatusCode: ctx.StatusCode,
						Message:    string(ctx.Body),
					}
				} else {
					apiError = APIError{
						StatusCode: ctx.StatusCode,
						Message:    http.StatusText(ctx.StatusCode),
					}
				}
				ctx.Error = &apiError
			}

			return next(ctx)
		}
	}
}

// ResponseLoggingMiddleware logs response details using the client's logger
func ResponseLoggingMiddleware(logger Logger) ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			logger.Debug("Response for %s %s: Status %d, Body length: %d bytes",
				ctx.Request.Method, ctx.Request.Path, ctx.StatusCode, len(ctx.Body))

			if ctx.Error != nil {
				logger.Debug("Response error: %v", ctx.Error)
			}

			return next(ctx)
		}
	}
}

// MetricsMiddleware could collect metrics about requests/responses
func MetricsMiddleware(logger Logger) ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			// Here you could collect metrics like:
			// - Response time
			// - Status codes
			// - Error rates
			// - Request/response sizes

			// For now, just log basic metrics
			logger.Debug("Metrics: %s %s -> %d (%d bytes)",
				ctx.Request.Method, ctx.Request.Path, ctx.StatusCode, len(ctx.Body))

			return next(ctx)
		}
	}
}

// CacheMiddleware could implement response caching
func CacheMiddleware() ResponseMiddleware {
	return func(next ResponseHandler) ResponseHandler {
		return func(ctx *ResponseContext) error {
			// Cache implementation would go here
			// For now, just pass through
			return next(ctx)
		}
	}
}
