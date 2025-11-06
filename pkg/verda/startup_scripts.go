package verda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type StartupScriptService struct {
	client *Client
}

type CreateStartupScriptRequest struct {
	Name   string `json:"name"`
	Script string `json:"script"`
}

// Get retrieves all startup scripts
func (s *StartupScriptService) Get(ctx context.Context) ([]StartupScript, error) {
	scripts, _, err := getRequest[[]StartupScript](ctx, s.client, "/scripts")
	if err != nil {
		return nil, err
	}
	return scripts, nil
}

// GetByID fetches a specific startup script by its ID
// The API returns an array with a single script object, so we extract it
func (s *StartupScriptService) GetByID(ctx context.Context, id string) (*StartupScript, error) {
	path := fmt.Sprintf("/scripts/%s", id)

	// API returns an array, so we request as array and extract first element
	scripts, _, err := getRequest[[]StartupScript](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	if len(scripts) == 0 {
		return nil, fmt.Errorf("script not found: %s", id)
	}

	return &scripts[0], nil
}

// Create creates a new startup script
// The API returns a plain text ID, so we fetch the full object after creation
func (s *StartupScriptService) Create(ctx context.Context, req CreateStartupScriptRequest) (*StartupScript, error) {
	// Try to create using the standard request library first
	script, _, err := postRequest[StartupScript](ctx, s.client, "/scripts", req)
	if err != nil {
		// If that fails, the API might be returning plain text instead of JSON
		return s.createWithPlainTextResponse(ctx, req)
	}
	return &script, nil
}

// createWithPlainTextResponse handles the case where the API returns plain text ID instead of JSON
func (s *StartupScriptService) createWithPlainTextResponse(ctx context.Context, req CreateStartupScriptRequest) (*StartupScript, error) {
	resp, err := s.client.makeRequest(ctx, http.MethodPost, "/scripts", req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiError APIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    string(body),
			}
		}
		apiError.StatusCode = resp.StatusCode
		return nil, &apiError
	}

	// Try to parse as JSON first
	var script StartupScript
	if err := json.Unmarshal(body, &script); err == nil {
		return &script, nil
	}

	// If JSON parsing fails, assume it's a plain text ID
	scriptID := strings.TrimSpace(string(body))
	// Remove quotes if present
	scriptID = strings.Trim(scriptID, "\"")

	// Fetch the full script details
	return s.GetByID(ctx, scriptID)
}

// Delete removes a startup script
func (s *StartupScriptService) Delete(ctx context.Context, id string) error {
	path := fmt.Sprintf("/scripts/%s", id)
	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}
