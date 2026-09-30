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

//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// TestPrivateNetworks exercises the private network lifecycle: create, list,
// get, update, subnet CRUD, route listing. Route creation is not covered: it
// requires a deployed instance with a public IP in the subnet's datacenter.
func TestPrivateNetworks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// Pick a location where private networks are available
	locations, err := client.Locations.Get(ctx)
	if err != nil {
		t.Fatalf("failed to get locations: %v", err)
	}
	locationCode := ""
	for _, loc := range locations {
		if loc.IsPrivateNetworksEnabled {
			locationCode = loc.Code
			break
		}
	}
	if locationCode == "" {
		t.Skip("no location with private networks enabled")
	}

	// High random /24 to dodge existing ranges in the project
	cidr := fmt.Sprintf("10.%d.%d.0/24", 200+rand.Intn(40), rand.Intn(200))

	network, err := client.PrivateNetworks.Create(ctx, verda.CreatePrivateNetworkRequest{
		Name: generateRandomName("sdk-test-net"),
		Mode: verda.NetworkModeCustom,
	})
	if err != nil {
		t.Fatalf("failed to create private network: %v", err)
	}
	t.Logf("Created private network %s (%s)", network.ID, network.Name)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := client.PrivateNetworks.Delete(cleanupCtx, network.ID); err != nil {
			t.Logf("⚠️  Failed to delete private network %s: %v", network.ID, err)
		}
	})

	t.Run("get_network", func(t *testing.T) {
		fetched, err := client.PrivateNetworks.GetByID(ctx, network.ID)
		if err != nil {
			t.Fatalf("failed to get network: %v", err)
		}
		if fetched.Name != network.Name {
			t.Errorf("expected name %q, got %q", network.Name, fetched.Name)
		}
	})

	t.Run("list_networks", func(t *testing.T) {
		networks, err := client.PrivateNetworks.Get(ctx)
		if err != nil {
			t.Fatalf("failed to list networks: %v", err)
		}
		found := false
		for _, n := range networks {
			if n.ID == network.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("created network %s not present in list", network.ID)
		}
	})

	t.Run("update_network", func(t *testing.T) {
		renamed := network.Name + "-renamed"
		updated, err := client.PrivateNetworks.Update(ctx, network.ID, verda.UpdatePrivateNetworkRequest{Name: &renamed})
		if err != nil {
			t.Fatalf("failed to update network: %v", err)
		}
		if updated.Name != renamed {
			t.Errorf("expected name %q, got %q", renamed, updated.Name)
		}
	})

	subnet, err := client.PrivateNetworks.CreateSubnet(ctx, network.ID, verda.CreateSubnetRequest{
		LocationCode: locationCode,
		Name:         generateRandomName("sdk-test-subnet"),
		CIDR:         cidr,
	})
	if err != nil {
		t.Fatalf("failed to create subnet in %s (%s): %v", locationCode, cidr, err)
	}
	t.Logf("Created subnet %s (%s) in %s", subnet.ID, subnet.CIDR, subnet.Location.Code)

	t.Run("get_subnet", func(t *testing.T) {
		fetched, err := client.PrivateNetworks.GetSubnetByID(ctx, network.ID, subnet.ID)
		if err != nil {
			t.Fatalf("failed to get subnet: %v", err)
		}
		if fetched.CIDR != cidr {
			t.Errorf("expected CIDR %q, got %q", cidr, fetched.CIDR)
		}
		if fetched.TotalIPCount == 0 {
			t.Error("expected non-zero total_ip_count")
		}
	})

	t.Run("list_subnets", func(t *testing.T) {
		subnets, err := client.PrivateNetworks.GetSubnets(ctx, network.ID)
		if err != nil {
			t.Fatalf("failed to list subnets: %v", err)
		}
		if len(subnets) == 0 {
			t.Error("expected at least one subnet")
		}
	})

	t.Run("update_subnet", func(t *testing.T) {
		renamed := subnet.Name + "-renamed"
		updated, err := client.PrivateNetworks.UpdateSubnet(ctx, network.ID, subnet.ID, verda.UpdateSubnetRequest{Name: &renamed})
		if err != nil {
			t.Fatalf("failed to update subnet: %v", err)
		}
		if updated.Name != renamed {
			t.Errorf("expected name %q, got %q", renamed, updated.Name)
		}
		if updated.CIDR != cidr {
			t.Errorf("CIDR must not change: expected %q, got %q", cidr, updated.CIDR)
		}
	})

	t.Run("list_routes_empty", func(t *testing.T) {
		routes, err := client.PrivateNetworks.GetRoutes(ctx, network.ID, subnet.ID)
		if err != nil {
			t.Fatalf("failed to list routes: %v", err)
		}
		if len(routes) != 0 {
			t.Errorf("expected no routes on a fresh subnet, got %d", len(routes))
		}
	})

	t.Run("delete_subnet", func(t *testing.T) {
		if err := client.PrivateNetworks.DeleteSubnet(ctx, network.ID, subnet.ID); err != nil {
			t.Fatalf("failed to delete subnet: %v", err)
		}
	})
}
