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

// KubernetesService manages managed Kubernetes clusters. It is experimental:
// the public routes may change while the API is finalized.
//
// Every call is scoped to an explicit project. Mutations return once the
// request is accepted; poll Get to observe provisioning, upgrades and deletion.
type KubernetesService struct {
	client *Client
}

// kubernetesClustersBasePath is the cluster collection route; every
// Kubernetes path is built from it
const kubernetesClustersBasePath = "/mk8sclusters"

// kubernetesClusterPath builds /mk8sclusters[/{name}]?projectId=...
func kubernetesClusterPath(projectID string, name ...string) string {
	path := kubernetesClustersBasePath
	for _, ch := range name {
		path += "/" + url.PathEscape(ch)
	}
	params := url.Values{}
	params.Set("projectId", projectID)
	return fmt.Sprintf("%s?%s", path, params.Encode())
}

// Create requests a new cluster. The returned cluster is normally still
// provisioning.
func (s *KubernetesService) Create(ctx context.Context, projectID string, req CreateKubernetesClusterRequest) (*KubernetesCluster, error) {
	if err := validateKubernetesSelectors(projectID); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	cluster, _, err := postRequest[KubernetesCluster](ctx, s.client, kubernetesClusterPath(projectID), req)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (s *KubernetesService) Get(ctx context.Context, projectID, name string) (*KubernetesCluster, error) {
	if err := validateKubernetesSelectors(projectID, name); err != nil {
		return nil, err
	}

	cluster, _, err := getRequest[KubernetesCluster](ctx, s.client, kubernetesClusterPath(projectID, name))
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

// Update requests a Kubernetes version upgrade. Success means the new
// version was accepted, not that the upgrade finished.
func (s *KubernetesService) Update(ctx context.Context, projectID, name string, req UpdateKubernetesClusterRequest) (*KubernetesCluster, error) {
	if err := validateKubernetesSelectors(projectID, name); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	cluster, _, err := patchRequest[KubernetesCluster](ctx, s.client, kubernetesClusterPath(projectID, name), req)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

// Delete requests cluster removal, which continues asynchronously
func (s *KubernetesService) Delete(ctx context.Context, projectID, name string) error {
	if err := validateKubernetesSelectors(projectID, name); err != nil {
		return err
	}

	_, err := deleteRequestAllowEmptyResponse(ctx, s.client, kubernetesClusterPath(projectID, name))
	return err
}
