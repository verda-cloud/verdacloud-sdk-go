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
)

type PrivateNetworkService struct {
	client *Client
}

// privateNetworkPath returns /private-networks/{networkID} followed by suffix.
// Every caller-supplied segment is validated and escaped.
func privateNetworkPath(networkID, suffix string) (string, error) {
	return joinPath("/private-networks", "networkID", networkID, suffix)
}

// subnetPath returns /private-networks/{networkID}/subnets/{subnetID}
// followed by suffix
func subnetPath(networkID, subnetID, suffix string) (string, error) {
	base, err := privateNetworkPath(networkID, "/subnets")
	if err != nil {
		return "", err
	}
	return joinPath(base, "subnetID", subnetID, suffix)
}

// routePath returns /private-networks/{networkID}/subnets/{subnetID}/routes/{routeID}
func routePath(networkID, subnetID, routeID string) (string, error) {
	base, err := subnetPath(networkID, subnetID, "/routes")
	if err != nil {
		return "", err
	}
	return joinPath(base, "routeID", routeID, "")
}

func (s *PrivateNetworkService) Create(ctx context.Context, req CreatePrivateNetworkRequest) (*PrivateNetwork, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	network, _, err := postRequest[PrivateNetwork](ctx, s.client, "/private-networks", req)
	if err != nil {
		return nil, err
	}
	return &network, nil
}

func (s *PrivateNetworkService) Get(ctx context.Context) ([]PrivateNetwork, error) {
	networks, _, err := getRequest[[]PrivateNetwork](ctx, s.client, "/private-networks")
	if err != nil {
		return nil, err
	}
	return networks, nil
}

func (s *PrivateNetworkService) GetByID(ctx context.Context, id string) (*PrivateNetwork, error) {
	path, err := privateNetworkPath(id, "")
	if err != nil {
		return nil, err
	}

	network, _, err := getRequest[PrivateNetwork](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &network, nil
}

// Update renames a network or marks it the project's default
func (s *PrivateNetworkService) Update(ctx context.Context, id string, req UpdatePrivateNetworkRequest) (*PrivateNetwork, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := privateNetworkPath(id, "")
	if err != nil {
		return nil, err
	}

	network, _, err := patchRequest[PrivateNetwork](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &network, nil
}

// Delete deletes a private network and its subnets
func (s *PrivateNetworkService) Delete(ctx context.Context, id string) error {
	path, err := privateNetworkPath(id, "")
	if err != nil {
		return err
	}
	_, err = deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}

// GetWorkloads lists the instances attached to a network, newest first.
// The list does not include cluster nodes.
func (s *PrivateNetworkService) GetWorkloads(ctx context.Context, networkID string) ([]PrivateNetworkWorkload, error) {
	path, err := privateNetworkPath(networkID, "/workloads")
	if err != nil {
		return nil, err
	}

	workloads, _, err := getRequest[[]PrivateNetworkWorkload](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return workloads, nil
}

// GetSubnetWorkloads lists the instances that take their private address
// from a subnet, newest first. The list does not include cluster nodes.
func (s *PrivateNetworkService) GetSubnetWorkloads(ctx context.Context, networkID, subnetID string) ([]PrivateNetworkWorkload, error) {
	path, err := subnetPath(networkID, subnetID, "/workloads")
	if err != nil {
		return nil, err
	}

	workloads, _, err := getRequest[[]PrivateNetworkWorkload](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return workloads, nil
}

func (s *PrivateNetworkService) CreateSubnet(ctx context.Context, networkID string, req CreateSubnetRequest) (*Subnet, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := privateNetworkPath(networkID, "/subnets")
	if err != nil {
		return nil, err
	}

	subnet, _, err := postRequest[Subnet](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &subnet, nil
}

func (s *PrivateNetworkService) GetSubnets(ctx context.Context, networkID string) ([]Subnet, error) {
	path, err := privateNetworkPath(networkID, "/subnets")
	if err != nil {
		return nil, err
	}

	subnets, _, err := getRequest[[]Subnet](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return subnets, nil
}

func (s *PrivateNetworkService) GetSubnetByID(ctx context.Context, networkID, subnetID string) (*Subnet, error) {
	path, err := subnetPath(networkID, subnetID, "")
	if err != nil {
		return nil, err
	}

	subnet, _, err := getRequest[Subnet](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &subnet, nil
}

// UpdateSubnet renames a subnet or marks it its datacenter's default.
// The subnet's CIDR cannot change.
func (s *PrivateNetworkService) UpdateSubnet(ctx context.Context, networkID, subnetID string, req UpdateSubnetRequest) (*Subnet, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := subnetPath(networkID, subnetID, "")
	if err != nil {
		return nil, err
	}

	subnet, _, err := patchRequest[Subnet](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &subnet, nil
}

func (s *PrivateNetworkService) DeleteSubnet(ctx context.Context, networkID, subnetID string) error {
	path, err := subnetPath(networkID, subnetID, "")
	if err != nil {
		return err
	}
	_, err = deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}

// CreateRoute routes a subnet's traffic for a destination through one of the
// network's instances. The instance must be in the subnet's datacenter and
// have a public IP.
func (s *PrivateNetworkService) CreateRoute(ctx context.Context, networkID, subnetID string, req CreateRouteRequest) (*Route, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path, err := subnetPath(networkID, subnetID, "/routes")
	if err != nil {
		return nil, err
	}

	route, _, err := postRequest[Route](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (s *PrivateNetworkService) GetRoutes(ctx context.Context, networkID, subnetID string) ([]Route, error) {
	path, err := subnetPath(networkID, subnetID, "/routes")
	if err != nil {
		return nil, err
	}

	routes, _, err := getRequest[[]Route](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return routes, nil
}

func (s *PrivateNetworkService) DeleteRoute(ctx context.Context, networkID, subnetID, routeID string) error {
	path, err := routePath(networkID, subnetID, routeID)
	if err != nil {
		return err
	}
	_, err = deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}
