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
	"encoding/json"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	t.Run("error with message", func(t *testing.T) {
		err := &APIError{
			StatusCode: 404,
			Message:    "Resource not found",
		}

		expected := "API error 404: Resource not found"
		if err.Error() != expected {
			t.Errorf("expected error message '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("error with details", func(t *testing.T) {
		err := &APIError{
			StatusCode: 400,
			Message:    "Bad request",
			Details:    "Invalid parameter",
		}

		expected := "API error 400: Bad request (Invalid parameter)"
		if err.Error() != expected {
			t.Errorf("expected error message '%s', got '%s'", expected, err.Error())
		}
	})
}

func TestAPIError_UnmarshalJSON(t *testing.T) {
	t.Run("without details", func(t *testing.T) {
		body := []byte(`{"code": "not_found", "message": "cluster not found"}`)

		var apiError APIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		if apiError.Code != "not_found" || apiError.Message != "cluster not found" || apiError.Details != "" {
			t.Errorf("unexpected decode result: %+v", apiError)
		}
	})

	t.Run("string details", func(t *testing.T) {
		body := []byte(`{"code": "invalid_request", "message": "Bad request", "details": "Invalid parameter"}`)

		var apiError APIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		if apiError.Details != "Invalid parameter" {
			t.Errorf("expected details 'Invalid parameter', got '%s'", apiError.Details)
		}
	})

	t.Run("object details", func(t *testing.T) {
		body := []byte(`{"code": "conflict", "message": "cluster is being deleted", "details": {"reason": "cluster_deleting"}}`)

		var apiError APIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		if apiError.Code != "conflict" || apiError.Message != "cluster is being deleted" {
			t.Errorf("unexpected decode result: %+v", apiError)
		}
		if apiError.Details != "cluster_deleting" {
			t.Errorf("expected details 'cluster_deleting', got '%s'", apiError.Details)
		}
	})

	t.Run("null details", func(t *testing.T) {
		body := []byte(`{"code": "server_error", "message": "internal", "details": null}`)

		var apiError APIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		if apiError.Details != "" {
			t.Errorf("expected empty details, got '%s'", apiError.Details)
		}
	})
}

func TestValidationError_Error(t *testing.T) {
	t.Run("validation error", func(t *testing.T) {
		err := &ValidationError{
			Field:   "email",
			Message: "Invalid email format",
		}

		expected := "validation error for field 'email': Invalid email format"
		if err.Error() != expected {
			t.Errorf("expected error message '%s', got '%s'", expected, err.Error())
		}
	})
}
