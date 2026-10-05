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
	"fmt"
	"strings"

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

// TagFilter matches resources carrying a tag. An empty Value matches the key
// with any value, otherwise the value must match exactly.
type TagFilter struct {
	Key   string
	Value string
}

// String returns the filter in the API query format: "key" or "key=value".
func (filter TagFilter) String() string {
	if filter.Value == "" {
		return filter.Key
	}
	return filter.Key + "=" + filter.Value
}

// Validate validates the TagFilter fields
func (filter TagFilter) Validate() error {
	return validation.ValidateStruct(&filter,
		validation.Field(&filter.Key, validation.Required, validation.Length(0, TagKeyMaxLength), validation.By(validateTagFilterKey)),
		validation.Field(&filter.Value, validation.Length(0, TagValueMaxLength)),
	)
}

// The API splits filters at the first "=", so a key containing one is ambiguous.
func validateTagFilterKey(value any) error {
	key, _ := value.(string)
	if strings.Contains(key, "=") {
		return fmt.Errorf("must not contain '=', got %q", key)
	}
	return nil
}
