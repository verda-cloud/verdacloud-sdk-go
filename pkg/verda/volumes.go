package verda

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type VolumeService struct {
	client *Client
}

func (s *VolumeService) ListVolumes(ctx context.Context) ([]Volume, error) {
	return s.ListVolumesByStatus(ctx, "")
}

func (s *VolumeService) ListVolumesByStatus(ctx context.Context, status string) ([]Volume, error) {
	path := "/volumes"
	if status != "" {
		params := url.Values{}
		params.Set("status", status)
		path += "?" + params.Encode()
	}

	volumes, _, err := getRequest[[]Volume](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return volumes, nil
}

func (s *VolumeService) GetVolume(ctx context.Context, id string) (*Volume, error) {
	path := fmt.Sprintf("/volumes/%s", id)
	volume, _, err := getRequest[Volume](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &volume, nil
}

func (s *VolumeService) CreateVolume(ctx context.Context, req VolumeCreateRequest) (string, error) {
	if req.LocationCode == "" {
		req.LocationCode = LocationFIN01
	}

	return s.createVolumeWithPlainTextResponse(ctx, req)
}

func (s *VolumeService) createVolumeWithPlainTextResponse(ctx context.Context, req VolumeCreateRequest) (string, error) {
	resp, err := s.client.makeRequest(ctx, http.MethodPost, "/volumes", req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", s.client.handleResponse(resp, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	volumeID := strings.TrimSpace(string(body))
	return volumeID, nil
}

func (s *VolumeService) DeleteVolume(ctx context.Context, id string, force bool) error {
	path := fmt.Sprintf("/volumes/%s", id)
	if force {
		path += "?force=true"
	}

	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}

// AttachVolume attaches a volume - instance must be shut down first
func (s *VolumeService) AttachVolume(ctx context.Context, volumeID string, req VolumeAttachRequest) error {
	path := fmt.Sprintf("/volumes/%s/attach", volumeID)
	_, err := postRequestNoResult(ctx, s.client, path, req)
	return err
}

// DetachVolume detaches a volume - instance must be shut down first
func (s *VolumeService) DetachVolume(ctx context.Context, volumeID string, req VolumeDetachRequest) error {
	path := fmt.Sprintf("/volumes/%s/detach", volumeID)
	_, err := postRequestNoResult(ctx, s.client, path, req)
	return err
}

func (s *VolumeService) CloneVolume(ctx context.Context, volumeID string, req VolumeCloneRequest) (string, error) {
	path := fmt.Sprintf("/volumes/%s/clone", volumeID)

	resp, err := s.client.makeRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", s.client.handleResponse(resp, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	newVolumeID := strings.TrimSpace(string(body))
	return newVolumeID, nil
}

// ResizeVolume grows a volume - shrinking is not supported
func (s *VolumeService) ResizeVolume(ctx context.Context, volumeID string, req VolumeResizeRequest) error {
	path := fmt.Sprintf("/volumes/%s/resize", volumeID)
	_, err := postRequestNoResult(ctx, s.client, path, req)
	return err
}

func (s *VolumeService) RenameVolume(ctx context.Context, volumeID string, req VolumeRenameRequest) error {
	path := fmt.Sprintf("/volumes/%s/rename", volumeID)
	_, err := postRequestNoResult(ctx, s.client, path, req)
	return err
}
