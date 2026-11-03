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
	"fmt"
)

type APIError struct {
	StatusCode int    `json:"status_code,omitempty"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

// UnmarshalJSON accepts details as either a string or an object with a
// reason string; API services send both shapes.
func (e *APIError) UnmarshalJSON(data []byte) error {
	type apiErrorAlias APIError

	aux := struct {
		*apiErrorAlias
		Details json.RawMessage `json:"details"`
	}{apiErrorAlias: (*apiErrorAlias)(e)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.Details) == 0 || string(aux.Details) == "null" {
		return nil
	}

	var details string
	if err := json.Unmarshal(aux.Details, &details); err == nil {
		e.Details = details
		return nil
	}

	var detailsObj struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(aux.Details, &detailsObj); err == nil {
		e.Details = detailsObj.Reason
	}

	return nil
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("API error %d: %s (%s)", e.StatusCode, e.Message, e.Details)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}
