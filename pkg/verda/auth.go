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
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// tokenExpiryMargin is how long before expiry a token is treated as expired, to avoid races
const tokenExpiryMargin = 30 * time.Second

type AuthService struct {
	client *Client
	mu     sync.RWMutex // guards token
	token  *TokenResponse

	// tokenSem serializes token requests; unlike a mutex, waiting for it can be canceled
	tokenSemOnce sync.Once
	tokenSem     chan struct{}
}

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	ExpiresAt    time.Time
}

// Authenticate requests a new token with client credentials.
//
// Deprecated: Use AuthenticateContext.
func (s *AuthService) Authenticate() (*TokenResponse, error) {
	return s.AuthenticateContext(context.Background())
}

// AuthenticateContext requests a new token with client credentials. ctx bounds both
// the wait for any in-flight token request and the token request itself.
func (s *AuthService) AuthenticateContext(ctx context.Context) (*TokenResponse, error) {
	release, err := s.acquireTokenLock(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	return s.doTokenRequest(ctx, s.clientCredentialsRequest())
}

// RefreshToken refreshes the current token, or authenticates if there is none.
//
// Deprecated: Use RefreshTokenContext.
func (s *AuthService) RefreshToken() (*TokenResponse, error) {
	return s.RefreshTokenContext(context.Background())
}

// RefreshTokenContext refreshes the current token, or authenticates if there is none.
// ctx bounds both the wait for any in-flight token request and the token request itself.
func (s *AuthService) RefreshTokenContext(ctx context.Context) (*TokenResponse, error) {
	release, err := s.acquireTokenLock(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	return s.refreshWithLock(ctx)
}

// refreshWithLock must be called while holding the token lock
func (s *AuthService) refreshWithLock(ctx context.Context) (*TokenResponse, error) {
	token := s.currentToken()
	if token == nil || token.RefreshToken == "" {
		return s.doTokenRequest(ctx, s.clientCredentialsRequest())
	}

	return s.doTokenRequest(ctx, TokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: token.RefreshToken,
		ClientID:     s.client.ClientID,
		ClientSecret: s.client.ClientSecret,
	})
}

func (s *AuthService) clientCredentialsRequest() TokenRequest {
	return TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     s.client.ClientID,
		ClientSecret: s.client.ClientSecret,
	}
}

// acquireTokenLock waits until no other token request is in flight, or until ctx is done
func (s *AuthService) acquireTokenLock(ctx context.Context) (release func(), err error) {
	s.tokenSemOnce.Do(func() {
		s.tokenSem = make(chan struct{}, 1)
	})

	select {
	case s.tokenSem <- struct{}{}:
		return func() { <-s.tokenSem }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for token request: %w", ctx.Err())
	}
}

func (s *AuthService) currentToken() *TokenResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

func (s *AuthService) setToken(token *TokenResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
}

func tokenExpired(token *TokenResponse) bool {
	return token == nil || time.Now().Add(tokenExpiryMargin).After(token.ExpiresAt)
}

// doTokenRequest tries JSON first (production), falls back to form-encoded (staging quirk)
func (s *AuthService) doTokenRequest(ctx context.Context, body TokenRequest) (*TokenResponse, error) {
	payload, err := json.Marshal(body) //nolint:gosec // G117: OAuth token request must include client_secret
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.client.BaseURL+"/oauth2/token", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if s.client.AuthBearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.client.AuthBearerToken)
	}

	resp, err := s.client.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("authentication request failed: %w", err)
	}

	var tokenResp TokenResponse
	err = s.client.handleResponse(resp, &tokenResp)
	if err == nil {
		tokenResp.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		s.setToken(&tokenResp)
		return &tokenResp, nil
	}

	// Staging environments may require form-encoded instead of JSON
	if apiErr, ok := err.(*APIError); ok {
		msg := strings.ToLower(apiErr.Message)
		if apiErr.StatusCode == 400 && (strings.Contains(msg, "grant_type") && strings.Contains(msg, "not specified") || strings.Contains(msg, "unsupported grant type") || strings.Contains(msg, "not valid json")) {
			form := url.Values{}
			form.Set("grant_type", body.GrantType)
			if body.ClientID != "" {
				form.Set("client_id", body.ClientID)
			}
			if body.ClientSecret != "" {
				form.Set("client_secret", body.ClientSecret)
			}
			if body.RefreshToken != "" {
				form.Set("refresh_token", body.RefreshToken)
			}

			req2, err2 := http.NewRequestWithContext(ctx, http.MethodPost, s.client.BaseURL+"/oauth2/token", strings.NewReader(form.Encode()))
			if err2 != nil {
				return nil, fmt.Errorf("failed to create token request (form): %w", err2)
			}
			req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req2.Header.Set("Accept", "application/json")
			if s.client.AuthBearerToken != "" {
				req2.Header.Set("Authorization", "Bearer "+s.client.AuthBearerToken)
			}
			resp2, err2 := s.client.HTTPClient.Do(req2)
			if err2 != nil {
				return nil, fmt.Errorf("authentication request failed (form): %w", err2)
			}
			var tokenResp2 TokenResponse
			err3 := s.client.handleResponse(resp2, &tokenResp2)
			if err3 == nil {
				tokenResp2.ExpiresAt = time.Now().Add(time.Duration(tokenResp2.ExpiresIn) * time.Second)
				s.setToken(&tokenResp2)
				return &tokenResp2, nil
			}
			// Report the cancellation rather than the JSON request's 400
			if ctx.Err() != nil {
				return nil, fmt.Errorf("authentication request failed (form): %w", err3)
			}
		}
	}
	return nil, fmt.Errorf("authentication failed: %w", err)
}

// GetValidToken returns the current token, authenticating or refreshing it when needed.
//
// Deprecated: Use GetValidTokenContext.
func (s *AuthService) GetValidToken() (*TokenResponse, error) {
	return s.GetValidTokenContext(context.Background())
}

// GetValidTokenContext returns the current token, authenticating or refreshing it when
// needed. Concurrent callers share a single token request. ctx bounds both the wait for
// an in-flight token request and the token request itself.
func (s *AuthService) GetValidTokenContext(ctx context.Context) (*TokenResponse, error) {
	if token := s.currentToken(); !tokenExpired(token) {
		return token, nil
	}

	release, err := s.acquireTokenLock(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	// Another caller may have obtained a token while this one waited
	if token := s.currentToken(); !tokenExpired(token) {
		return token, nil
	}

	return s.refreshWithLock(ctx)
}

func (s *AuthService) IsExpired() bool {
	return tokenExpired(s.currentToken())
}

// GetBearerToken returns the Authorization header value for the current token.
//
// Deprecated: Use GetBearerTokenContext.
func (s *AuthService) GetBearerToken() (string, error) {
	return s.GetBearerTokenContext(context.Background())
}

// GetBearerTokenContext returns the Authorization header value for the current token,
// authenticating or refreshing it when needed. See GetValidTokenContext.
func (s *AuthService) GetBearerTokenContext(ctx context.Context) (string, error) {
	token, err := s.GetValidTokenContext(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Bearer %s", token.AccessToken), nil
}
