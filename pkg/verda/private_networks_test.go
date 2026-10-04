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
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestPrivateNetworkService_Networks(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)
	ctx := context.Background()

	t.Run("create network", func(t *testing.T) {
		network, err := client.PrivateNetworks.Create(ctx, CreatePrivateNetworkRequest{
			Name:      "prod-net",
			Mode:      NetworkModeCustom,
			IsDefault: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if network.ID != "pn_new_123" {
			t.Errorf("expected network ID 'pn_new_123', got '%s'", network.ID)
		}
		if network.Name != "prod-net" {
			t.Errorf("expected name 'prod-net', got '%s'", network.Name)
		}
		if network.Mode != NetworkModeCustom {
			t.Errorf("expected mode '%s', got '%s'", NetworkModeCustom, network.Mode)
		}
		if !network.IsDefault {
			t.Error("expected is_default true")
		}
		if network.CreatedByUserID == "" || network.CreatedAt.IsZero() {
			t.Error("expected audit fields to be populated")
		}
	})

	t.Run("create network validation", func(t *testing.T) {
		if _, err := client.PrivateNetworks.Create(ctx, CreatePrivateNetworkRequest{}); err == nil {
			t.Error("expected error for missing name")
		}

		_, err := client.PrivateNetworks.Create(ctx, CreatePrivateNetworkRequest{Name: "x", Mode: "bogus"})
		if err == nil {
			t.Error("expected error for invalid mode")
		}
	})

	t.Run("list networks", func(t *testing.T) {
		networks, err := client.PrivateNetworks.Get(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(networks) != 1 || networks[0].ID != "pn_123" {
			t.Errorf("expected one network pn_123, got %+v", networks)
		}
	})

	t.Run("get network by ID", func(t *testing.T) {
		network, err := client.PrivateNetworks.GetByID(ctx, "pn_123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if network.ID != "pn_123" || network.Name != "prod-net" {
			t.Errorf("unexpected network: %+v", network)
		}
	})

	t.Run("update network", func(t *testing.T) {
		name := "renamed-net"
		isDefault := false
		network, err := client.PrivateNetworks.Update(ctx, "pn_123", UpdatePrivateNetworkRequest{
			Name:      &name,
			IsDefault: &isDefault,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if network.Name != "renamed-net" {
			t.Errorf("expected name 'renamed-net', got '%s'", network.Name)
		}
		if network.IsDefault {
			t.Error("expected is_default false")
		}
	})

	t.Run("update network validation", func(t *testing.T) {
		empty := ""
		_, err := client.PrivateNetworks.Update(ctx, "pn_123", UpdatePrivateNetworkRequest{Name: &empty})
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("delete network", func(t *testing.T) {
		if err := client.PrivateNetworks.Delete(ctx, "pn_123"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestPrivateNetworkService_Subnets(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)
	ctx := context.Background()

	t.Run("create subnet", func(t *testing.T) {
		subnet, err := client.PrivateNetworks.CreateSubnet(ctx, "pn_123", CreateSubnetRequest{
			LocationCode: "FIN-01",
			Name:         "workers",
			CIDR:         "10.0.0.0/24",
			IsDefault:    true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if subnet.ID != "sn_new_123" {
			t.Errorf("expected subnet ID 'sn_new_123', got '%s'", subnet.ID)
		}
		if subnet.Location.Code != "FIN-01" {
			t.Errorf("expected location 'FIN-01', got '%s'", subnet.Location.Code)
		}
		if subnet.CIDR != "10.0.0.0/24" || subnet.Name != "workers" {
			t.Errorf("unexpected subnet: %+v", subnet)
		}
		if subnet.TotalIPCount != 254 {
			t.Errorf("expected 254 total IPs, got %d", subnet.TotalIPCount)
		}
	})

	t.Run("create subnet validation", func(t *testing.T) {
		if _, err := client.PrivateNetworks.CreateSubnet(ctx, "pn_123", CreateSubnetRequest{}); err == nil {
			t.Error("expected error for missing required fields")
		}

		_, err := client.PrivateNetworks.CreateSubnet(ctx, "pn_123", CreateSubnetRequest{
			LocationCode: "FIN-01",
			Name:         "workers",
			CIDR:         "not-a-cidr",
		})
		if err == nil {
			t.Error("expected error for invalid CIDR")
		}

		_, err = client.PrivateNetworks.CreateSubnet(ctx, "pn_123", CreateSubnetRequest{
			LocationCode: "FIN-01",
			Name:         "workers",
			CIDR:         "fd00::/64",
		})
		if err == nil {
			t.Error("expected error for IPv6 CIDR")
		}
	})

	t.Run("list subnets", func(t *testing.T) {
		subnets, err := client.PrivateNetworks.GetSubnets(ctx, "pn_123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(subnets) != 1 || subnets[0].ID != "sn_123" {
			t.Errorf("expected one subnet sn_123, got %+v", subnets)
		}
	})

	t.Run("get subnet by ID", func(t *testing.T) {
		subnet, err := client.PrivateNetworks.GetSubnetByID(ctx, "pn_123", "sn_123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if subnet.ID != "sn_123" {
			t.Errorf("expected subnet ID 'sn_123', got '%s'", subnet.ID)
		}
	})

	t.Run("update subnet", func(t *testing.T) {
		name := "cpu-workers"
		subnet, err := client.PrivateNetworks.UpdateSubnet(ctx, "pn_123", "sn_123", UpdateSubnetRequest{Name: &name})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if subnet.Name != "cpu-workers" {
			t.Errorf("expected name 'cpu-workers', got '%s'", subnet.Name)
		}
	})

	t.Run("delete subnet", func(t *testing.T) {
		if err := client.PrivateNetworks.DeleteSubnet(ctx, "pn_123", "sn_123"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestPrivateNetworkService_Routes(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)
	ctx := context.Background()

	t.Run("create route", func(t *testing.T) {
		route, err := client.PrivateNetworks.CreateRoute(ctx, "pn_123", "sn_123", CreateRouteRequest{
			Destination: "0.0.0.0/0",
			InstanceID:  "inst_route_123",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if route.ID != "rt_new_123" {
			t.Errorf("expected route ID 'rt_new_123', got '%s'", route.ID)
		}
		if route.SubnetID != "sn_123" || route.Destination != "0.0.0.0/0" {
			t.Errorf("unexpected route: %+v", route)
		}
		if route.Instance == nil || route.Instance.ID != "inst_route_123" {
			t.Errorf("expected attached instance inst_route_123, got %+v", route.Instance)
		}
		if route.Instance.PublicIP == nil {
			t.Error("expected instance public IP")
		}
	})

	t.Run("create route validation", func(t *testing.T) {
		if _, err := client.PrivateNetworks.CreateRoute(ctx, "pn_123", "sn_123", CreateRouteRequest{}); err == nil {
			t.Error("expected error for missing required fields")
		}

		_, err := client.PrivateNetworks.CreateRoute(ctx, "pn_123", "sn_123", CreateRouteRequest{
			Destination: "everywhere",
			InstanceID:  "inst_123",
		})
		if err == nil {
			t.Error("expected error for invalid destination CIDR")
		}
	})

	t.Run("list routes", func(t *testing.T) {
		routes, err := client.PrivateNetworks.GetRoutes(ctx, "pn_123", "sn_123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(routes) != 1 || routes[0].ID != "rt_123" {
			t.Errorf("expected one route rt_123, got %+v", routes)
		}
	})

	t.Run("delete route", func(t *testing.T) {
		if err := client.PrivateNetworks.DeleteRoute(ctx, "pn_123", "sn_123", "rt_123"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestIPAllocation(t *testing.T) {
	t.Run("valid values", func(t *testing.T) {
		for _, alloc := range []IPAllocation{IPAllocAuto, IPAllocNone, "10.0.0.5", "192.168.1.1"} {
			if err := alloc.Validate(); err != nil {
				t.Errorf("expected %q to be valid, got %v", alloc, err)
			}
		}
	})

	t.Run("invalid values", func(t *testing.T) {
		for _, alloc := range []IPAllocation{"", "garbage", "10.0.0.300", "::1"} {
			if err := alloc.Validate(); err == nil {
				t.Errorf("expected %q to be invalid", alloc)
			}
		}
	})

	t.Run("IPAllocIPv4", func(t *testing.T) {
		if _, err := IPAllocIPv4("10.0.0.5"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if _, err := IPAllocIPv4("nope"); err == nil {
			t.Error("expected error for invalid IPv4")
		}
	})
}
