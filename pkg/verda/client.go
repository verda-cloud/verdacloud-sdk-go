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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"sync/atomic"
)

const (
	DefaultBaseURL = "https://api.verda.com/v1"
	trueString     = "true"
)

type Client struct {
	BaseURL         string
	ClientID        string
	ClientSecret    string
	AuthBearerToken string
	UserAgent       string

	HTTPClient *http.Client
	Logger     Logger

	// Middleware management for all requests
	Middleware *Middleware

	// Services
	Auth                 *AuthService
	Balance              *BalanceService
	Instances            *InstanceService
	Volumes              *VolumeService
	VolumeTypes          *VolumeTypeService
	SSHKeys              *SSHKeyService
	StartupScripts       *StartupScriptService
	Locations            *LocationService
	Images               *ImagesService
	InstanceTypes        *InstanceTypesService
	InstanceAvailability *InstanceAvailabilityService
	ContainerTypes       *ContainerTypesService
	Clusters             *ClusterService
	LongTerm             *LongTermService
	PrivateNetworks      *PrivateNetworkService
	ContainerDeployments *ContainerDeploymentsService
	ServerlessJobs       *ServerlessJobsService
	ManagedKubernetes    *ManagedKubernetesService
}

type ClientOption func(*Client)

func NewClient(options ...ClientOption) (*Client, error) {

	client := &Client{
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{},
		Logger:     &NoOpLogger{}, // Default: no logging
	}

	for _, option := range options {
		option(client)
	}

	// Enable debug logging via VERDA_DEBUG env var if no custom logger was set
	if verdaDebug := os.Getenv("VERDA_DEBUG"); strings.ToLower(verdaDebug) == trueString {
		if _, isNoOp := client.Logger.(*NoOpLogger); isNoOp {
			client.Logger = NewStdLogger(true)
		}
	}

	client.Middleware = NewDefaultMiddlewareWithUserAgent(client.Logger, client.UserAgent)

	// Wire up debug middleware if VERDA_DEBUG is set
	if verdaDebug := os.Getenv("VERDA_DEBUG"); strings.ToLower(verdaDebug) == trueString {
		client.Middleware.AddRequestMiddleware(DebugLoggingMiddleware(client.Logger))
		client.Middleware.AddResponseMiddleware(DebugResponseLoggingMiddleware(client.Logger))
	}

	if client.ClientID == "" {
		return nil, fmt.Errorf("client ID is required")
	}
	if client.ClientSecret == "" {
		return nil, fmt.Errorf("client secret is required")
	}

	client.Auth = newAuthService(client)
	client.Balance = &BalanceService{client: client}
	client.Instances = &InstanceService{client: client}
	client.Volumes = &VolumeService{client: client}
	client.VolumeTypes = &VolumeTypeService{client: client}
	client.SSHKeys = &SSHKeyService{client: client}
	client.StartupScripts = &StartupScriptService{client: client}
	client.Locations = &LocationService{client: client}
	client.Images = &ImagesService{client: client}
	client.InstanceTypes = &InstanceTypesService{client: client}
	client.InstanceAvailability = &InstanceAvailabilityService{client: client}
	client.ContainerTypes = &ContainerTypesService{client: client}
	client.Clusters = &ClusterService{client: client}
	client.LongTerm = &LongTermService{client: client}
	client.PrivateNetworks = &PrivateNetworkService{client: client}
	client.ContainerDeployments = &ContainerDeploymentsService{client: client}
	client.ServerlessJobs = &ServerlessJobsService{client: client}
	client.ManagedKubernetes = &ManagedKubernetesService{client: client}

	return client, nil
}

func (c *Client) AddRequestMiddleware(middleware RequestMiddleware) {
	c.Middleware.AddRequestMiddleware(middleware)
}

func (c *Client) AddResponseMiddleware(middleware ResponseMiddleware) {
	c.Middleware.AddResponseMiddleware(middleware)
}

func (c *Client) SetRequestMiddleware(middleware []RequestMiddleware) {
	c.Middleware.SetRequestMiddleware(middleware)
}

func (c *Client) SetResponseMiddleware(middleware []ResponseMiddleware) {
	c.Middleware.SetResponseMiddleware(middleware)
}

func (c *Client) ClearRequestMiddleware() {
	c.Middleware.ClearRequestMiddleware()
}

func (c *Client) ClearResponseMiddleware() {
	c.Middleware.ClearResponseMiddleware()
}

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

func WithClientID(clientID string) ClientOption {
	return func(c *Client) {
		c.ClientID = clientID
	}
}

func WithClientSecret(clientSecret string) ClientOption {
	return func(c *Client) {
		c.ClientSecret = clientSecret

	}
}

func WithAuthBearerToken(token string) ClientOption {
	return func(c *Client) {
		c.AuthBearerToken = token
	}
}

func WithLogger(logger Logger) ClientOption {
	return func(c *Client) {
		c.Logger = logger
	}
}

func WithDebugLogging(enabled bool) ClientOption {
	return func(c *Client) {
		c.Logger = NewStdLogger(enabled)
	}
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

// Deprecated: Duplicate function, will be removed in later release
func (c *Client) WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) {
		c.UserAgent = userAgent
	}
}

func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.BaseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req = req.WithContext(ctx)
	return req, nil
}

// Do executes the request through middleware and returns the parsed response.
//
// The request middleware chain wraps the real HTTP round trip: the chain's
// terminal handler sends the request, so a middleware that calls next sees the
// response (as an error for non-2xx statuses) and may call next again to retry.
func (c *Client) Do(req *http.Request, result any) (*Response, error) {
	// Snapshot middleware to avoid race conditions
	requestMiddleware, responseMiddleware := c.Middleware.Snapshot()

	reqCtx := &RequestContext{
		Method:  req.Method,
		Path:    req.URL.Path,
		Body:    nil,
		Headers: req.Header.Clone(),
		Query:   req.URL.Query(),
		Client:  c,
		Request: req,
	}

	// State of the attempts is local to this call, so concurrent calls never share it
	st := &doState{}
	terminal := c.attemptHandler(st, req, req.Header.Clone())
	chainErr := c.buildRequestChain(requestMiddleware, terminal)(reqCtx)

	if st.attempts == 0 {
		if chainErr != nil {
			return nil, fmt.Errorf("request middleware failed: %w", chainErr)
		}
		// A custom middleware returned without calling next: send once anyway
		chainErr = terminal(reqCtx)
	}

	if !st.isPlainOutcome(chainErr) {
		if st.lastErr != nil && errors.Is(chainErr, st.lastErr) {
			// The middleware wrapped the last attempt's error (cancellation, GetBody failure)
			return nil, chainErr
		}
		return nil, fmt.Errorf("request middleware failed: %w", chainErr)
	}

	// The last attempt got no response: there is nothing to parse
	if st.resp == nil {
		return nil, st.lastErr
	}

	wrappedResp := &Response{Response: st.resp}
	if st.readErr != nil {
		return wrappedResp, st.readErr
	}

	var parseErr error
	if st.apiErr != nil {
		parseErr = st.apiErr
	} else if result != nil {
		trimmed := bytes.TrimSpace(st.body)
		if len(trimmed) > 0 {
			if err := json.Unmarshal(trimmed, result); err != nil {
				parseErr = fmt.Errorf("failed to unmarshal response: %w", err)
			}
		}
	}

	respCtx := &ResponseContext{
		Request:    reqCtx,
		Response:   st.resp,
		Body:       st.body,
		StatusCode: st.resp.StatusCode,
		Error:      parseErr,
	}

	responseHandler := c.buildResponseChain(responseMiddleware)
	if middlewareErr := responseHandler(respCtx); middlewareErr != nil {
		return wrappedResp, fmt.Errorf("response middleware failed: %w", middlewareErr)
	}

	if respCtx.Error != nil {
		return wrappedResp, respCtx.Error
	}

	return wrappedResp, nil
}

// doState holds the outcome of the latest attempt of one Do call.
// Each attempt resets it, so a result is never carried over from an earlier attempt.
type doState struct {
	attempts int
	resp     *http.Response
	body     []byte
	apiErr   *APIError
	lastErr  error // *transportError or *attemptError of the latest attempt
	readErr  error
}

// isPlainOutcome reports whether chainErr only carries the outcome of the
// latest attempt, as opposed to an error a middleware produced itself.
func (st *doState) isPlainOutcome(chainErr error) bool {
	if chainErr == nil || chainErr == st.lastErr {
		return true
	}
	var exhausted *retryExhaustedError
	return errors.As(chainErr, &exhausted) && exhausted == chainErr && exhausted.last == st.lastErr
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

// attemptHandler returns the terminal handler of the request chain: one real HTTP round trip.
// initialHeader is the request's header as the caller gave it: a name that was
// there and is gone from reqCtx.Headers was deleted by a middleware.
func (c *Client) attemptHandler(st *doState, orig *http.Request, initialHeader http.Header) RequestHandler {
	return func(reqCtx *RequestContext) error {
		st.attempts++
		st.resp, st.body, st.apiErr, st.lastErr, st.readErr = nil, nil, nil, nil, nil

		r := reqCtx.Request
		if r == nil {
			r = orig
		}

		// Count written requests: zero writes on a failed attempt means the server saw nothing
		var writes atomic.Int32
		trace := &httptrace.ClientTrace{
			WroteRequest: func(httptrace.WroteRequestInfo) { writes.Add(1) },
		}
		attemptReq := r.Clone(httptrace.WithClientTrace(r.Context(), trace))
		attemptReq.Header = attemptHeader(r.Header, initialHeader, reqCtx.Headers)

		hc, track := c.attemptClient()
		var origURL string
		if attemptReq.URL != nil { // a middleware may have swapped in a request without one
			origURL = attemptReq.URL.String()
		}

		resp, err := hc.Do(attemptReq) //nolint:gosec // G704: SDK client - URL is configured by caller
		if err != nil {
			st.lastErr = &transportError{
				err: err, writes: writes.Load(), redirected: track.redirected.Load(), redirectDenied: track.denied(err),
				origURL: origURL, method: attemptReq.Method,
			}
			return st.lastErr
		}
		st.resp = resp

		body, readErr := readAndCloseResponseBody(resp)
		if readErr != nil {
			st.readErr = readErr
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				st.lastErr = &transportError{err: readErr, writes: writes.Load(), redirected: track.redirected.Load(), origURL: origURL, method: attemptReq.Method}
				return st.lastErr
			}
			body = nil
		}
		st.body = body

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			st.apiErr = parseAPIError(resp.StatusCode, body)
			st.lastErr = &attemptError{
				apiErr:           st.apiErr,
				header:           resp.Header,
				originalExchange: !track.redirected.Load() && resp.Request != nil && resp.Request.Response == nil,
				method:           attemptReq.Method,
			}
			return st.lastErr
		}

		return nil
	}
}

// attemptHeader builds the header of one attempt. A header a middleware set
// directly on the request is kept, a header it deleted from reqCtx.Headers is
// dropped, and every header in reqCtx.Headers wins, as it did before attempts existed.
func attemptHeader(onRequest, initial, fromContext http.Header) http.Header {
	header := onRequest.Clone()
	if header == nil {
		header = make(http.Header)
	}
	for name := range initial {
		if _, kept := fromContext[name]; !kept {
			header.Del(name)
		}
	}
	for name, values := range fromContext {
		header[name] = append([]string(nil), values...)
	}
	return header
}

// defaultMaxRedirects is the redirect limit of net/http's default policy.
const defaultMaxRedirects = 10

// attemptClient returns a shallow copy of the client's HTTP client for one attempt.
// Its CheckRedirect records that a redirect happened, then applies the caller's
// policy (or Go's default), so the shared client is never changed.
func (c *Client) attemptClient() (*http.Client, *redirectTracker) {
	base := c.HTTPClient
	if base == nil {
		base = http.DefaultClient
	}
	hc := *base
	track := &redirectTracker{}
	userCheck := base.CheckRedirect
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		track.redirected.Store(true)
		var err error
		switch {
		case userCheck != nil:
			err = userCheck(req, via)
		case len(via) >= defaultMaxRedirects:
			err = fmt.Errorf("stopped after %d redirects", defaultMaxRedirects)
		}
		if err != nil && !errors.Is(err, http.ErrUseLastResponse) {
			track.policyErr.Store(&err)
		}
		return err
	}
	return &hc, track
}

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

func (c *Client) buildRequestChain(requestMiddleware []RequestMiddleware, terminal RequestHandler) RequestHandler {
	handler := terminal

	// Reverse order so last middleware wraps first
	for i := len(requestMiddleware) - 1; i >= 0; i-- {
		handler = requestMiddleware[i](handler)
	}

	return handler
}

func (c *Client) buildResponseChain(responseMiddleware []ResponseMiddleware) ResponseHandler {
	//nolint:revive // ctx unused in default no-op handler
	handler := func(ctx *ResponseContext) error {
		return nil
	}

	// Reverse order so last middleware wraps first
	for i := len(responseMiddleware) - 1; i >= 0; i-- {
		handler = responseMiddleware[i](handler)
	}

	return handler
}

// makeRequest is kept for endpoints with plain text responses that don't fit the normal JSON flow.
// Prefer NewRequest() + Do() for new code to get full middleware support.
func (c *Client) makeRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	url := c.BaseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", BuildUserAgent(c.UserAgent))

	bearerToken, err := c.Auth.GetBearerTokenContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication token: %w", err)
	}
	req.Header.Set("Authorization", bearerToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func (c *Client) handleResponse(resp *http.Response, result any) error {
	body, err := readAndCloseResponseBody(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, body)
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}

	if result != nil {
		if err := json.Unmarshal(trimmed, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

func readAndCloseResponseBody(resp *http.Response) ([]byte, error) {
	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		readErr = fmt.Errorf("failed to read response body: %w", readErr)
		if closeErr != nil {
			return nil, errors.Join(readErr, fmt.Errorf("failed to close response body: %w", closeErr))
		}
		return nil, readErr
	}
	return body, nil
}
