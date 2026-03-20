package verda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	sshKeysPath       = "/ssh-keys"
	sshKeysLegacyPath = "/sshkeys"
)

type SSHKeyService struct {
	client *Client
}

type CreateSSHKeyRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"key"`
}

type DeleteMultipleSSHKeysRequest struct {
	Keys []string `json:"keys"`
}

func (s *SSHKeyService) GetAllSSHKeys(ctx context.Context) ([]SSHKey, error) {
	return s.getAllSSHKeysFromPath(ctx, sshKeysPath)
}

func (s *SSHKeyService) GetSSHKeyByID(ctx context.Context, sshKeyID string) (*SSHKey, error) {
	return s.getSSHKeyByIDFromPath(ctx, fmt.Sprintf("%s/%s", sshKeysPath, sshKeyID))
}

// AddSSHKey creates a key and refetches it since the API returns only the ID as plain text
func (s *SSHKeyService) AddSSHKey(ctx context.Context, req *CreateSSHKeyRequest) (*SSHKey, error) {
	return s.createWithPlainTextResponse(ctx, req)
}

func (s *SSHKeyService) createWithPlainTextResponse(ctx context.Context, req *CreateSSHKeyRequest) (*SSHKey, error) {
	resp, err := s.client.makeRequest(ctx, http.MethodPost, sshKeysPath, req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read error response: %w", err)
		}
		if resp.StatusCode == http.StatusNotFound {
			return s.createWithPlainTextResponseFromPath(ctx, req, sshKeysLegacyPath)
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	keyID := strings.TrimSpace(string(body))
	return s.GetSSHKeyByID(ctx, keyID)
}

func (s *SSHKeyService) DeleteSSHKey(ctx context.Context, sshKeyID string) error {
	path := fmt.Sprintf("%s/%s", sshKeysPath, sshKeyID)
	_, err := deleteRequestAllowEmptyResponse(ctx, s.client, path)
	if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == http.StatusNotFound {
		fallbackPath := fmt.Sprintf("%s/%s", sshKeysLegacyPath, sshKeyID)
		_, err = deleteRequestAllowEmptyResponse(ctx, s.client, fallbackPath)
	}
	return err
}

func (s *SSHKeyService) DeleteMultipleSSHKeys(ctx context.Context, keyIDs []string) error {
	req := &DeleteMultipleSSHKeysRequest{
		Keys: keyIDs,
	}
	_, err := deleteRequestWithBody(ctx, s.client, sshKeysPath, req)
	if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == http.StatusNotFound {
		_, err = deleteRequestWithBody(ctx, s.client, sshKeysLegacyPath, req)
	}
	return err
}

func (s *SSHKeyService) getAllSSHKeysFromPath(ctx context.Context, path string) ([]SSHKey, error) {
	keys, _, err := getRequest[[]SSHKey](ctx, s.client, path)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == http.StatusNotFound && path == sshKeysPath {
			return s.getAllSSHKeysFromPath(ctx, sshKeysLegacyPath)
		}
		return nil, err
	}
	return keys, nil
}

func (s *SSHKeyService) getSSHKeyByIDFromPath(ctx context.Context, path string) (*SSHKey, error) {
	resp, err := s.client.makeRequest(ctx, http.MethodGet, path, nil)
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

	if resp.StatusCode == http.StatusNotFound && strings.HasPrefix(path, sshKeysPath+"/") {
		fallbackPath := strings.Replace(path, sshKeysPath, sshKeysLegacyPath, 1)
		return s.getSSHKeyByIDFromPath(ctx, fallbackPath)
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

	key, err := parseSSHKeyResponse(body)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (s *SSHKeyService) createWithPlainTextResponseFromPath(ctx context.Context, req *CreateSSHKeyRequest, path string) (*SSHKey, error) {
	resp, err := s.client.makeRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read error response: %w", err)
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	keyID := strings.TrimSpace(string(body))
	return s.GetSSHKeyByID(ctx, keyID)
}

func parseSSHKeyResponse(body []byte) (*SSHKey, error) {
	var key SSHKey
	if err := json.Unmarshal(body, &key); err == nil && key.ID != "" {
		return &key, nil
	}

	var keys []SSHKey
	if err := json.Unmarshal(body, &keys); err == nil {
		if len(keys) == 0 {
			return nil, fmt.Errorf("SSH key not found")
		}
		return &keys[0], nil
	}

	return nil, fmt.Errorf("unexpected SSH key response format")
}
