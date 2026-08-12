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
	"context"
	"fmt"
	"net/url"
)

// addResourceTag adds a single tag to the resource at basePath/resourceID.
// Shared by the instance, volume and cluster tag endpoints, which are identical
// apart from their base path.
func addResourceTag(ctx context.Context, client *Client, basePath, resourceID string, req TagRequest) (*Tag, error) {
	if resourceID == "" {
		return nil, fmt.Errorf("resource ID is required")
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/%s/tags", basePath, url.PathEscape(resourceID))

	tag, _, err := postRequest[Tag](ctx, client, path, req)
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// deleteResourceTag removes the tag with the given key from the resource at
// basePath/resourceID.
func deleteResourceTag(ctx context.Context, client *Client, basePath, resourceID, key string) error {
	if resourceID == "" {
		return fmt.Errorf("resource ID is required")
	}

	if key == "" {
		return fmt.Errorf("tag key is required")
	}

	// Tag keys are arbitrary user-supplied text, so they must be escaped before
	// being interpolated into the request path.
	path := fmt.Sprintf("%s/%s/tags/%s", basePath, url.PathEscape(resourceID), url.PathEscape(key))

	_, err := deleteRequestAllowEmptyResponse(ctx, client, path)
	return err
}
