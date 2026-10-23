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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// maxRawErrorMessageLength bounds the Message taken from a non-JSON error
// body such as a proxy's HTML page
const maxRawErrorMessageLength = 1024

type APIError struct {
	StatusCode int    `json:"status_code,omitempty"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("API error %d: %s (%s)", e.StatusCode, e.Message, e.Details)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// errorEnvelope accepts the error bodies the API returns:
// {"code","message","details"}, and {"error"} from some auth layers.
// Fields are decoded loosely because message can also be a list of strings.
type errorEnvelope struct {
	Code    json.RawMessage `json:"code"`
	Message json.RawMessage `json:"message"`
	Error   json.RawMessage `json:"error"`
	Details json.RawMessage `json:"details"`
}

// parseAPIError builds an APIError from a non-2xx response. Known JSON
// envelopes are decoded; anything else becomes a bounded raw-text message.
func parseAPIError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		apiErr.Message = http.StatusText(statusCode)
		return apiErr
	}

	var envelope errorEnvelope
	if trimmed[0] == '{' && json.Unmarshal(trimmed, &envelope) == nil {
		apiErr.Code = jsonText(envelope.Code)
		apiErr.Details = jsonText(envelope.Details)
		apiErr.Message = jsonText(envelope.Message)
		if apiErr.Message == "" {
			apiErr.Message = jsonText(envelope.Error)
		}
	}
	if apiErr.Message == "" {
		apiErr.Message = truncateErrorText(string(trimmed))
	}
	return apiErr
}

// jsonText renders a JSON value as text: strings as-is, lists of strings
// joined with "; ", and any other value as compact JSON
func jsonText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return strings.Join(list, "; ")
	}
	var compact bytes.Buffer
	if json.Compact(&compact, raw) == nil {
		return compact.String()
	}
	return string(raw)
}

func truncateErrorText(s string) string {
	s = strings.ToValidUTF8(s, "�")
	if len(s) <= maxRawErrorMessageLength {
		return s
	}
	cut := maxRawErrorMessageLength
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}
