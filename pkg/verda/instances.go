package verda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type InstanceService struct {
	client *Client
}

func (s *InstanceService) Get(ctx context.Context, status string) ([]Instance, error) {
	path := "/instances"
	if status != "" {
		params := url.Values{}
		params.Set("status", status)
		path += "?" + params.Encode()
	}

	instances, _, err := getRequest[[]Instance](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (s *InstanceService) GetByID(ctx context.Context, id string) (*Instance, error) {
	path := fmt.Sprintf("/instances/%s", id)

	instance, _, err := getRequest[Instance](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

func (s *InstanceService) Create(ctx context.Context, req CreateInstanceRequest) (*Instance, error) {
	if req.LocationCode == "" {
		req.LocationCode = LocationFIN01
	}

	if req.SSHKeyIDs == nil {
		req.SSHKeyIDs = []string{}
	}

	return s.createWithPlainTextResponse(ctx, req)
}

// createWithPlainTextResponse handles API's inconsistent response format (sometimes JSON, sometimes plain text ID)
func (s *InstanceService) createWithPlainTextResponse(ctx context.Context, req CreateInstanceRequest) (*Instance, error) {
	resp, err := s.client.makeRequest(ctx, http.MethodPost, "/instances", req)
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
		s.client.Logger.Debug("Instance creation failed with status %d: %s", resp.StatusCode, string(body))

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var instance Instance
	if err := json.Unmarshal(body, &instance); err != nil {
		// Fall back to plain text ID
		instanceID := strings.TrimSpace(string(body))
		return s.GetByID(ctx, instanceID)
	}

	return &instance, nil
}

func (s *InstanceService) Action(ctx context.Context, id string, action string, volumeIDs []string) error {
	req := InstanceActionRequest{
		ID:        id,
		Action:    action,
		VolumeIDs: volumeIDs,
	}

	_, _, err := putRequest[any](ctx, s.client, "/instances", req)
	return err
}

// Deprecated: Use CheckInstanceTypeAvailability
func (s *InstanceService) IsAvailable(ctx context.Context, instanceType string, isSpot bool, locationCode string) (bool, error) {
	path := fmt.Sprintf("/instance-availability/%s", instanceType)

	params := url.Values{}
	if isSpot {
		params.Set("is_spot", "true")
	}
	if locationCode != "" {
		params.Set("location_code", locationCode)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	// API returns "true"/"false" as JSON string, not boolean
	resp, err := s.client.makeRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiError APIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			return false, &APIError{
				StatusCode: resp.StatusCode,
				Message:    string(body),
			}
		}
		apiError.StatusCode = resp.StatusCode
		return false, &apiError
	}

	var boolResult bool
	if err := json.Unmarshal(body, &boolResult); err == nil {
		return boolResult, nil
	}

	var stringResult string
	if err := json.Unmarshal(body, &stringResult); err == nil {
		return stringResult == trueString, nil
	}

	return false, fmt.Errorf("unexpected response format: %s", string(body))
}

// Deprecated: Use GetLocationAvailabilities
func (s *InstanceService) GetAvailabilities(ctx context.Context, isSpot *bool, locationCode string) ([]InstanceAvailability, error) {
	path := "/instance-availability"

	params := url.Values{}
	if isSpot != nil {
		if *isSpot {
			params.Set("is_spot", "true")
		} else {
			params.Set("is_spot", "false")
		}
	}
	if locationCode != "" {
		params.Set("location_code", locationCode)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	availabilities, _, err := getRequest[[]InstanceAvailability](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return availabilities, nil
}

func (s *InstanceService) GetLocationAvailabilities(ctx context.Context) ([]LocationAvailability, error) {
	availabilities, _, err := getRequest[[]LocationAvailability](ctx, s.client, "/instance-availability")
	if err != nil {
		return nil, err
	}

	return availabilities, nil
}

func (s *InstanceService) CheckInstanceTypeAvailability(ctx context.Context, instanceType string) (bool, error) {
	path := fmt.Sprintf("/instance-availability/%s", instanceType)

	// API returns "true"/"false" as JSON string, not boolean
	resp, err := s.client.makeRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiError APIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			return false, &APIError{
				StatusCode: resp.StatusCode,
				Message:    string(body),
			}
		}
		apiError.StatusCode = resp.StatusCode
		return false, &apiError
	}

	var boolResult bool
	if err := json.Unmarshal(body, &boolResult); err == nil {
		return boolResult, nil
	}

	var stringResult string
	if err := json.Unmarshal(body, &stringResult); err == nil {
		return stringResult == trueString, nil
	}

	return false, fmt.Errorf("unexpected response format: %s", string(body))
}

func (s *InstanceService) Boot(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionBoot, nil)
}

func (s *InstanceService) Start(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionStart, nil)
}

func (s *InstanceService) Shutdown(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionShutdown, nil)
}

func (s *InstanceService) Delete(ctx context.Context, id string, volumeIDs []string) error {
	return s.Action(ctx, id, ActionDelete, volumeIDs)
}

func (s *InstanceService) Discontinue(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionDiscontinue, nil)
}

// Hibernate shuts down and archives an instance - must be shut down first or API will error.
// Volumes are detached and the instance is deleted during hibernation.
func (s *InstanceService) Hibernate(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionHibernate, nil)
}

func (s *InstanceService) ConfigureSpot(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionConfigureSpot, nil)
}

func (s *InstanceService) ForceShutdown(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionForceShutdown, nil)
}

func (s *InstanceService) DeleteStuck(ctx context.Context, id string, volumeIDs []string) error {
	return s.Action(ctx, id, ActionDeleteStuck, volumeIDs)
}

func (s *InstanceService) Deploy(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionDeploy, nil)
}

func (s *InstanceService) Transfer(ctx context.Context, id string) error {
	return s.Action(ctx, id, ActionTransfer, nil)
}
