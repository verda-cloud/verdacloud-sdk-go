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

// validation.go contains shared validation helpers used across multiple service
// type files (e.g. container_deployments_types.go, serverless_jobs_types.go).
//
// Service-specific validation logic (Validate methods, validateCreate* functions)
// lives in the corresponding *_types.go file. Only put helpers here when they
// are reused by more than one service domain.

package verda

import "strings"

// IsLatestTag checks if a container image uses the "latest" tag.
// The API does not allow "latest" tag - a specific version must be used.
func IsLatestTag(image string) bool {
	if strings.HasSuffix(image, ":latest") {
		return true
	}
	if !strings.Contains(image, ":") && !strings.Contains(image, "@") {
		return true
	}
	return false
}
