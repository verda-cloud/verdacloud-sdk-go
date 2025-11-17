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

type DeleteMultipleStartupScriptsRequest struct {
	Scripts []string `json:"scripts"`
}

func (s *StartupScriptService) GetAllStartupScripts(ctx context.Context) ([]StartupScript, error) {
	scripts, _, err := getRequest[[]StartupScript](ctx, s.client, "/scripts")
	if err != nil {
		return nil, err
	}
	return scripts, nil
}

func (s *StartupScriptService) GetStartupScriptByID(ctx context.Context, scriptID string) (*StartupScript, error) {
	path := fmt.Sprintf("/scripts/%s", scriptID)

	// API returns array even for single script lookup
	scripts, _, err := getRequest[[]StartupScript](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	if len(scripts) == 0 {
		return nil, fmt.Errorf("script not found: %s", scriptID)
	}

	return &scripts[0], nil
}

// AddStartupScript creates a script and refetches it since the API returns only the ID as plain text
func (s *StartupScriptService) AddStartupScript(ctx context.Context, req *CreateStartupScriptRequest) (*StartupScript, error) {
	return s.createWithPlainTextResponse(ctx, req)
}

// createWithPlainTextResponse handles API's inconsistent response format (sometimes JSON, sometimes plain text ID)
func (s *StartupScriptService) createWithPlainTextResponse(ctx context.Context, req *CreateStartupScriptRequest) (*StartupScript, error) {
	resp, err := s.client.makeRequest(ctx, http.MethodPost, "/scripts", req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

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

	// Try JSON first, fall back to plain text ID
	var script StartupScript
	if err := json.Unmarshal(body, &script); err == nil {
		return &script, nil
	}

	scriptID := strings.TrimSpace(string(body))
	scriptID = strings.Trim(scriptID, "\"")

	return s.GetStartupScriptByID(ctx, scriptID)
}

func (s *StartupScriptService) DeleteStartupScript(ctx context.Context, scriptID string) error {
	path := fmt.Sprintf("/scripts/%s", scriptID)
	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}

func (s *StartupScriptService) DeleteMultipleStartupScripts(ctx context.Context, scriptIDs []string) error {
	req := &DeleteMultipleStartupScriptsRequest{
		Scripts: scriptIDs,
	}
	_, err := deleteRequestWithBody(ctx, s.client, "/scripts", req)
	return err
}

// Deprecated: Use GetAllStartupScripts
func (s *StartupScriptService) Get(ctx context.Context) ([]StartupScript, error) {
	return s.GetAllStartupScripts(ctx)
}

// Deprecated: Use GetStartupScriptByID
func (s *StartupScriptService) GetByID(ctx context.Context, id string) (*StartupScript, error) {
	return s.GetStartupScriptByID(ctx, id)
}

// Deprecated: Use AddStartupScript
func (s *StartupScriptService) Create(ctx context.Context, req CreateStartupScriptRequest) (*StartupScript, error) {
	return s.AddStartupScript(ctx, &req)
}

// Deprecated: Use DeleteStartupScript
func (s *StartupScriptService) Delete(ctx context.Context, id string) error {
	return s.DeleteStartupScript(ctx, id)
}
