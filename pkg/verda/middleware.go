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
