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

// MK8SService manages managed Kubernetes clusters. This service is experimental.
// The public routes may change before the API becomes final.
//
// Every call takes an explicit project ID. Mutations return after the API
// accepts the request. Poll Get to check provisioning, upgrades, and deletion.
type MK8SService struct {
	client *Client
}

// mk8sClustersBasePath is the cluster collection route.
// Every MK8S path uses this route.
const mk8sClustersBasePath = "/mk8sclusters"

// mk8sClusterPath builds /mk8sclusters[/{name}]?projectId=...
func mk8sClusterPath(projectID string, name ...string) string {
	path := mk8sClustersBasePath
	for _, ch := range name {
		path += "/" + url.PathEscape(ch)
	}
	params := url.Values{}
	params.Set("projectId", projectID)
	return fmt.Sprintf("%s?%s", path, params.Encode())
}

// Create requests a new cluster. The returned cluster is normally still
// provisioning.
func (s *MK8SService) Create(ctx context.Context, projectID string, req CreateMK8SClusterRequest) (*MK8SCluster, error) {
	if err := validateMK8SSelectors(projectID); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	cluster, _, err := postRequest[MK8SCluster](ctx, s.client, mk8sClusterPath(projectID), req)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (s *MK8SService) Get(ctx context.Context, projectID, name string) (*MK8SCluster, error) {
	if err := validateMK8SSelectors(projectID, name); err != nil {
		return nil, err
	}

	cluster, _, err := getRequest[MK8SCluster](ctx, s.client, mk8sClusterPath(projectID, name))
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

// Update requests a Kubernetes version upgrade. Success means the API
// accepted the new version. It does not mean that the upgrade finished.
func (s *MK8SService) Update(ctx context.Context, projectID, name string, req UpdateMK8SClusterRequest) (*MK8SCluster, error) {
	if err := validateMK8SSelectors(projectID, name); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	cluster, _, err := patchRequest[MK8SCluster](ctx, s.client, mk8sClusterPath(projectID, name), req)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

// Delete requests cluster removal, which continues asynchronously
func (s *MK8SService) Delete(ctx context.Context, projectID, name string) error {
	if err := validateMK8SSelectors(projectID, name); err != nil {
		return err
	}

	_, err := deleteRequestAllowEmptyResponse(ctx, s.client, mk8sClusterPath(projectID, name))
	return err
}
