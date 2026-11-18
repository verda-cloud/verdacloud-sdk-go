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
	"net/url"
)

// pathSegment validates and escapes one caller-supplied URL path segment,
// such as a resource ID or name, so that it stays a single segment.
// An empty value, ".", and ".." are rejected: they survive escaping and would
// change which endpoint is called (an empty ID turns "GET /resources/{id}"
// into the list endpoint).
func pathSegment(field, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", field)
	}
	if value == "." || value == ".." {
		return "", fmt.Errorf("%s %q is not a valid path segment", field, value)
	}
	return url.PathEscape(value), nil
}

// joinPath returns base + "/" + the validated, escaped value + suffix.
// field labels the value in errors.
func joinPath(base, field, value, suffix string) (string, error) {
	segment, err := pathSegment(field, value)
	if err != nil {
		return "", err
	}
	return base + "/" + segment + suffix, nil
}
