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
)

type PrivateNetworkService struct {
	client *Client
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
	path := fmt.Sprintf("/private-networks/%s", id)

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

	path := fmt.Sprintf("/private-networks/%s", id)

	network, _, err := patchRequest[PrivateNetwork](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &network, nil
}

// Delete deletes a private network and its subnets
func (s *PrivateNetworkService) Delete(ctx context.Context, id string) error {
	path := fmt.Sprintf("/private-networks/%s", id)
	_, err := deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}

func (s *PrivateNetworkService) CreateSubnet(ctx context.Context, networkID string, req CreateSubnetRequest) (*Subnet, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/private-networks/%s/subnets", networkID)

	subnet, _, err := postRequest[Subnet](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &subnet, nil
}

func (s *PrivateNetworkService) GetSubnets(ctx context.Context, networkID string) ([]Subnet, error) {
	path := fmt.Sprintf("/private-networks/%s/subnets", networkID)

	subnets, _, err := getRequest[[]Subnet](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return subnets, nil
}

func (s *PrivateNetworkService) GetSubnetByID(ctx context.Context, networkID, subnetID string) (*Subnet, error) {
	path := fmt.Sprintf("/private-networks/%s/subnets/%s", networkID, subnetID)

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

	path := fmt.Sprintf("/private-networks/%s/subnets/%s", networkID, subnetID)

	subnet, _, err := patchRequest[Subnet](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &subnet, nil
}

func (s *PrivateNetworkService) DeleteSubnet(ctx context.Context, networkID, subnetID string) error {
	path := fmt.Sprintf("/private-networks/%s/subnets/%s", networkID, subnetID)
	_, err := deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}

// CreateRoute routes a subnet's traffic for a destination through one of the
// network's instances. The instance must be in the subnet's datacenter and
// have a public IP.
func (s *PrivateNetworkService) CreateRoute(ctx context.Context, networkID, subnetID string, req CreateRouteRequest) (*Route, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/private-networks/%s/subnets/%s/routes", networkID, subnetID)

	route, _, err := postRequest[Route](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (s *PrivateNetworkService) GetRoutes(ctx context.Context, networkID, subnetID string) ([]Route, error) {
	path := fmt.Sprintf("/private-networks/%s/subnets/%s/routes", networkID, subnetID)

	routes, _, err := getRequest[[]Route](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return routes, nil
}

func (s *PrivateNetworkService) DeleteRoute(ctx context.Context, networkID, subnetID, routeID string) error {
	path := fmt.Sprintf("/private-networks/%s/subnets/%s/routes/%s", networkID, subnetID, routeID)
	_, err := deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}
