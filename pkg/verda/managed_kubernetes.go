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

const managedKubernetesBasePath = "/mk8sclusters"

type ManagedKubernetesService struct {
	client *Client
}

// clusterPath returns the API path of a cluster. Every user-supplied path
// segment is validated and escaped so that it stays a single segment.
func (s *ManagedKubernetesService) clusterPath(location, clusterName string) (string, error) {
	loc, err := managedKubernetesPathSegment("location", location)
	if err != nil {
		return "", err
	}
	if err := validateManagedKubernetesName("clusterName", clusterName, 63); err != nil {
		return "", err
	}
	return managedKubernetesBasePath + "/" + loc + "/" + url.PathEscape(clusterName), nil
}

func (s *ManagedKubernetesService) nodePoolsPath(location, clusterName string) (string, error) {
	base, err := s.clusterPath(location, clusterName)
	if err != nil {
		return "", err
	}
	return base + "/pools", nil
}

func (s *ManagedKubernetesService) nodePoolPath(location, clusterName, poolName string) (string, error) {
	base, err := s.nodePoolsPath(location, clusterName)
	if err != nil {
		return "", err
	}
	if err := validateManagedKubernetesName("poolName", poolName, 30); err != nil {
		return "", err
	}
	return base + "/" + url.PathEscape(poolName), nil
}

func (s *ManagedKubernetesService) kubeconfigsPath(location, clusterName string) (string, error) {
	base, err := s.clusterPath(location, clusterName)
	if err != nil {
		return "", err
	}
	return base + "/kubeconfigs", nil
}

func (s *ManagedKubernetesService) kubeconfigPath(location, clusterName, id string) (string, error) {
	base, err := s.kubeconfigsPath(location, clusterName)
	if err != nil {
		return "", err
	}
	segment, err := managedKubernetesPathSegment("id", id)
	if err != nil {
		return "", err
	}
	return base + "/" + segment, nil
}

// managedKubernetesPathSegment validates and escapes a free-form path segment.
// "." and ".." survive path escaping and would change which endpoint is called.
func managedKubernetesPathSegment(field, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", field)
	}
	if value == "." || value == ".." {
		return "", fmt.Errorf("%s %q is not a valid path segment", field, value)
	}
	return url.PathEscape(value), nil
}

func (s *ManagedKubernetesService) List(ctx context.Context) ([]ManagedKubernetesCluster, error) {
	clusters, _, err := getRequest[[]ManagedKubernetesCluster](ctx, s.client, managedKubernetesBasePath)
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

func (s *ManagedKubernetesService) Get(ctx context.Context, location, clusterName string) (*ManagedKubernetesCluster, error) {
	path, err := s.clusterPath(location, clusterName)
	if err != nil {
		return nil, err
	}

	cluster, _, err := getRequest[ManagedKubernetesCluster](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (s *ManagedKubernetesService) Create(ctx context.Context, req CreateManagedKubernetesClusterRequest) (*ManagedKubernetesCluster, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	cluster, _, err := postRequest[ManagedKubernetesCluster](ctx, s.client, managedKubernetesBasePath, req)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (s *ManagedKubernetesService) Update(ctx context.Context, location, clusterName string, req UpdateManagedKubernetesClusterRequest) (*ManagedKubernetesCluster, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := s.clusterPath(location, clusterName)
	if err != nil {
		return nil, err
	}

	cluster, _, err := patchRequest[ManagedKubernetesCluster](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

// Delete schedules the cluster for deletion (async; poll Get until it
// returns an *APIError with StatusCode 404).
func (s *ManagedKubernetesService) Delete(ctx context.Context, location, clusterName string) error {
	path, err := s.clusterPath(location, clusterName)
	if err != nil {
		return err
	}

	_, err = deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}

func (s *ManagedKubernetesService) GetEnabledLocations(ctx context.Context) (*ManagedKubernetesEnabledLocations, error) {
	path := managedKubernetesBasePath + "/enabled_locations"

	locations, _, err := getRequest[ManagedKubernetesEnabledLocations](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &locations, nil
}

func (s *ManagedKubernetesService) ListNodePools(ctx context.Context, location, clusterName string) ([]ManagedKubernetesNodePool, error) {
	path, err := s.nodePoolsPath(location, clusterName)
	if err != nil {
		return nil, err
	}

	pools, _, err := getRequest[[]ManagedKubernetesNodePool](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return pools, nil
}

func (s *ManagedKubernetesService) GetNodePool(ctx context.Context, location, clusterName, poolName string) (*ManagedKubernetesNodePool, error) {
	path, err := s.nodePoolPath(location, clusterName, poolName)
	if err != nil {
		return nil, err
	}

	pool, _, err := getRequest[ManagedKubernetesNodePool](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (s *ManagedKubernetesService) CreateNodePool(ctx context.Context, location, clusterName string, req CreateManagedKubernetesNodePoolRequest) (*ManagedKubernetesNodePool, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := s.nodePoolsPath(location, clusterName)
	if err != nil {
		return nil, err
	}

	pool, _, err := postRequest[ManagedKubernetesNodePool](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (s *ManagedKubernetesService) UpdateNodePool(ctx context.Context, location, clusterName, poolName string, req UpdateManagedKubernetesNodePoolRequest) (*ManagedKubernetesNodePool, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := s.nodePoolPath(location, clusterName, poolName)
	if err != nil {
		return nil, err
	}

	pool, _, err := patchRequest[ManagedKubernetesNodePool](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

// DeleteNodePool schedules the pool for deletion (async; poll GetNodePool
// until it returns an *APIError with StatusCode 404).
func (s *ManagedKubernetesService) DeleteNodePool(ctx context.Context, location, clusterName, poolName string) error {
	path, err := s.nodePoolPath(location, clusterName, poolName)
	if err != nil {
		return err
	}

	_, err = deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}

// ListKubeconfigs returns the cluster's kubeconfigs with metadata and status
// only; the rendered YAML is never included. Use GetKubeconfig for content.
func (s *ManagedKubernetesService) ListKubeconfigs(ctx context.Context, location, clusterName string) ([]ManagedKubernetesKubeconfig, error) {
	path, err := s.kubeconfigsPath(location, clusterName)
	if err != nil {
		return nil, err
	}

	kubeconfigs, _, err := getRequest[[]ManagedKubernetesKubeconfig](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return kubeconfigs, nil
}

// GetKubeconfig returns one kubeconfig. The rendered YAML in the Kubeconfig
// field, and NotAfter, are present only once Status is ready.
func (s *ManagedKubernetesService) GetKubeconfig(ctx context.Context, location, clusterName, id string) (*ManagedKubernetesKubeconfig, error) {
	path, err := s.kubeconfigPath(location, clusterName, id)
	if err != nil {
		return nil, err
	}

	kubeconfig, _, err := getRequest[ManagedKubernetesKubeconfig](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &kubeconfig, nil
}

// CreateKubeconfig issues a client certificate and returns the new
// kubeconfig record with Status pending; poll GetKubeconfig until ready.
func (s *ManagedKubernetesService) CreateKubeconfig(ctx context.Context, location, clusterName string, req CreateManagedKubernetesKubeconfigRequest) (*ManagedKubernetesKubeconfig, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := s.kubeconfigsPath(location, clusterName)
	if err != nil {
		return nil, err
	}

	kubeconfig, _, err := postRequest[ManagedKubernetesKubeconfig](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &kubeconfig, nil
}

// DeleteKubeconfig schedules the kubeconfig for deletion (async; 202).
// Already-downloaded copies are not revoked and keep working until expiry.
func (s *ManagedKubernetesService) DeleteKubeconfig(ctx context.Context, location, clusterName, id string) error {
	path, err := s.kubeconfigPath(location, clusterName, id)
	if err != nil {
		return err
	}

	_, err = deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}
