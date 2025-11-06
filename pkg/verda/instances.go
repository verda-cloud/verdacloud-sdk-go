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

// Get retrieves all non-deleted instances, optionally filtered by status
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

// GetByID fetches a specific instance by its ID
func (s *InstanceService) GetByID(ctx context.Context, id string) (*Instance, error) {
	path := fmt.Sprintf("/instances/%s", id)

	instance, _, err := getRequest[Instance](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// Create creates and deploys a new cloud instance
func (s *InstanceService) Create(ctx context.Context, req CreateInstanceRequest) (*Instance, error) {
	// Set default location if not provided
	if req.LocationCode == "" {
		req.LocationCode = LocationFIN01
	}

	// Ensure SSH key IDs is not nil
	if req.SSHKeyIDs == nil {
		req.SSHKeyIDs = []string{}
	}

	// Try to create using the new request library first
	// The API might return either JSON or just an instance ID as plain text
	instance, _, err := postRequest[Instance](ctx, s.client, "/instances", req)
	if err != nil {
		// If the new request library fails, it might be because the API returned plain text
		// In this case, we need to handle it manually

		// For now, let's try to extract the instance ID from the error and fetch the instance
		// This is a fallback for APIs that return plain text instead of JSON

		// Try to make a raw request to handle the plain text response
		return s.createWithPlainTextResponse(ctx, req)
	}

	return &instance, nil
}

// createWithPlainTextResponse handles the case where the API returns plain text instead of JSON
func (s *InstanceService) createWithPlainTextResponse(ctx context.Context, req CreateInstanceRequest) (*Instance, error) {
	// Use the old method as a fallback for plain text responses
	resp, err := s.client.makeRequest(ctx, http.MethodPost, "/instances", req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error in defer
	}()

	// Check for error status codes first
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read and parse the error response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read error response: %w", err)
		}
		s.client.Logger.Debug("Instance creation failed with status %d: %s", resp.StatusCode, string(body))

		// Parse the error manually since we already read the body
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var instance Instance
	if err := json.Unmarshal(body, &instance); err != nil {
		// If JSON parsing fails, assume it's just the instance ID as plain text
		instanceID := strings.TrimSpace(string(body))
		// Fetch the full instance details
		return s.GetByID(ctx, instanceID)
	}

	return &instance, nil
}

// Action performs actions on one or multiple instances
func (s *InstanceService) Action(ctx context.Context, idList any, action string, volumeIDs []string) error {
	req := InstanceActionRequest{
		IDList:    idList,
		Action:    action,
		VolumeIDs: volumeIDs,
	}

	_, _, err := postRequest[interface{}](ctx, s.client, "/instances/action", req)
	return err
}

// IsAvailable checks if a specific instance type is available
// Deprecated: Use CheckInstanceTypeAvailability instead
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

	// The API returns a JSON string "true" or "false", not a boolean
	// We need to handle this manually
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

	// Try to parse as boolean first
	var boolResult bool
	if err := json.Unmarshal(body, &boolResult); err == nil {
		return boolResult, nil
	}

	// Try to parse as string "true" or "false"
	var stringResult string
	if err := json.Unmarshal(body, &stringResult); err == nil {
		return stringResult == "true", nil
	}

	return false, fmt.Errorf("unexpected response format: %s", string(body))
}

// GetAvailabilities gets available instance types across locations
// Deprecated: Use GetLocationAvailabilities instead. This method will be removed in a future version.
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

// GetLocationAvailabilities gets instance types available by location
func (s *InstanceService) GetLocationAvailabilities(ctx context.Context) ([]LocationAvailability, error) {
	availabilities, _, err := getRequest[[]LocationAvailability](ctx, s.client, "/instance-availability")
	if err != nil {
		return nil, err
	}

	return availabilities, nil
}

// CheckInstanceTypeAvailability checks if a specific instance type is available
func (s *InstanceService) CheckInstanceTypeAvailability(ctx context.Context, instanceType string) (bool, error) {
	path := fmt.Sprintf("/instance-availability/%s", instanceType)

	// The API returns a JSON string "true" or "false", not a boolean
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

	// Try to parse as boolean first
	var boolResult bool
	if err := json.Unmarshal(body, &boolResult); err == nil {
		return boolResult, nil
	}

	// Try to parse as string "true" or "false"
	var stringResult string
	if err := json.Unmarshal(body, &stringResult); err == nil {
		return stringResult == "true", nil
	}

	return false, fmt.Errorf("unexpected response format: %s", string(body))
}

// Delete is a convenience method to delete instances
func (s *InstanceService) Delete(ctx context.Context, idList any, volumeIDs []string) error {
	return s.Action(ctx, idList, ActionDelete, volumeIDs)
}

// Shutdown is a convenience method to shutdown instances
func (s *InstanceService) Shutdown(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ActionShutdown, nil)
}

// Hibernate is a convenience method to hibernate instances
func (s *InstanceService) Hibernate(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ActionHibernate, nil)
}
