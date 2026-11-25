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

import "context"

const instanceGroupsBasePath = "/instance-groups"

// InstanceGroupService manages externally controlled instance groups.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupService struct {
	client *Client
}

func instanceGroupPath(id string) (string, error) {
	return joinPath(instanceGroupsBasePath, "instanceGroupID", id, "")
}

func (s *InstanceGroupService) List(ctx context.Context) ([]InstanceGroup, error) {
	groups, _, err := getRequest[[]InstanceGroup](ctx, s.client, instanceGroupsBasePath)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *InstanceGroupService) GetByID(ctx context.Context, id string) (*InstanceGroup, error) {
	path, err := instanceGroupPath(id)
	if err != nil {
		return nil, err
	}

	group, _, err := getRequest[InstanceGroup](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (s *InstanceGroupService) Create(ctx context.Context, req CreateInstanceGroupRequest) (*InstanceGroup, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	group, _, err := postRequest[InstanceGroup](ctx, s.client, instanceGroupsBasePath, req)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (s *InstanceGroupService) Update(ctx context.Context, id string, req UpdateInstanceGroupRequest) (*InstanceGroup, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := instanceGroupPath(id)
	if err != nil {
		return nil, err
	}

	group, _, err := patchRequest[InstanceGroup](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// Delete soft-deletes a group. The API returns 409 while live group instances exist.
func (s *InstanceGroupService) Delete(ctx context.Context, id string) error {
	path, err := instanceGroupPath(id)
	if err != nil {
		return err
	}
	_, err = deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}
