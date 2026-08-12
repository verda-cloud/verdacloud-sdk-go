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
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Tag limits enforced by the API
const (
	// TagKeyMaxLength is the maximum length of a tag key
	TagKeyMaxLength = 63
	// TagValueMaxLength is the maximum length of a tag value
	TagValueMaxLength = 127
	// MaxTagsPerResource is the maximum number of tags a single resource may carry
	MaxTagsPerResource = 10
)

// Tag represents a key-value tag attached to a resource
type Tag struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TagRequest represents a tag to add to a resource
type TagRequest struct {
	Key string `json:"key"`
	// Value is omitted from the request when empty, which creates a freeform tag
	Value string `json:"value,omitempty"`
}

// Validate validates the TagRequest fields
func (r TagRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Key, validation.Required, validation.Length(0, TagKeyMaxLength)),
		validation.Field(&r.Value, validation.Length(0, TagValueMaxLength)),
	)
}
