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

package integration

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// TestPrivateNetworks exercises the private network lifecycle: create, list,
// get, update, subnet CRUD, route listing. It also checks these API rules:
//   - a network created without a mode gets mode "custom";
//   - the location reports private networks and an IPv4 auto subnet;
//   - an explicit is_default=false is honoured, an omitted one makes the
//     first default subnet, and setting the default moves it;
//   - both workload endpoints return an empty list for a fresh network, and
//     a subnet of another network is not found;
//   - an unknown network is not found.
//
// Successful route creation and non-empty workloads are not covered: they
// require a deployed instance in the subnet's datacenter.
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

	notDefault := false
	subnet, err := client.PrivateNetworks.CreateSubnet(ctx, network.ID, verda.CreateSubnetRequest{
		LocationCode: locationCode,
		Name:         generateRandomName("sdk-test-subnet"),
		CIDR:         cidr,
		IsDefault:    &notDefault,
	})
	if err != nil {
		t.Fatalf("failed to create subnet in %s (%s): %v", locationCode, cidr, err)
	}
	t.Logf("Created subnet %s (%s) in %s", subnet.ID, subnet.CIDR, subnet.Location.Code)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// The delete_subnet subtest normally removes it first
		if err := client.PrivateNetworks.DeleteSubnet(cleanupCtx, network.ID, subnet.ID); err != nil && !isNotFound(err) {
			t.Logf("⚠️  Failed to delete subnet %s: %v", subnet.ID, err)
		}
	})

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

	t.Run("first_subnet_explicit_not_default", func(t *testing.T) {
		if subnet.IsDefault {
			t.Error("expected the create response to honor is_default=false")
		}
		fetched, err := client.PrivateNetworks.GetSubnetByID(ctx, network.ID, subnet.ID)
		if err != nil {
			t.Fatalf("failed to get subnet: %v", err)
		}
		if fetched.IsDefault {
			t.Error("expected is_default=false to persist")
		}
	})

	// The next /24 after the first subnet, in the same location
	octets := netip.MustParsePrefix(cidr).Addr().As4()
	octets[2]++
	secondCIDR := netip.PrefixFrom(netip.AddrFrom4(octets), 24).String()
	second, err := client.PrivateNetworks.CreateSubnet(ctx, network.ID, verda.CreateSubnetRequest{
		LocationCode: locationCode,
		Name:         generateRandomName("sdk-test-subnet"),
		CIDR:         secondCIDR,
	})
	if err != nil {
		t.Fatalf("failed to create second subnet in %s (%s): %v", locationCode, secondCIDR, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := client.PrivateNetworks.DeleteSubnet(cleanupCtx, network.ID, second.ID); err != nil {
			t.Logf("⚠️  Failed to delete subnet %s: %v", second.ID, err)
		}
	})

	t.Run("second_subnet_omitted_default", func(t *testing.T) {
		// The location has no default subnet yet, so the API makes this one the default
		if !second.IsDefault {
			t.Error("expected a subnet created without is_default to become the default")
		}
	})

	t.Run("update_subnet_default_swaps", func(t *testing.T) {
		isDefault := true
		updated, err := client.PrivateNetworks.UpdateSubnet(ctx, network.ID, subnet.ID, verda.UpdateSubnetRequest{IsDefault: &isDefault})
		if err != nil {
			t.Fatalf("failed to update subnet: %v", err)
		}
		if !updated.IsDefault {
			t.Error("expected the first subnet to become the default")
		}
		fetched, err := client.PrivateNetworks.GetSubnetByID(ctx, network.ID, second.ID)
		if err != nil {
			t.Fatalf("failed to get second subnet: %v", err)
		}
		if fetched.IsDefault {
			t.Error("expected the second subnet to lose the default")
		}
	})

	t.Run("workloads_empty", func(t *testing.T) {
		workloads, err := client.PrivateNetworks.GetWorkloads(ctx, network.ID)
		if err != nil {
			t.Fatalf("failed to list network workloads: %v", err)
		}
		if len(workloads) != 0 {
			t.Errorf("expected no workloads on a fresh network, got %d", len(workloads))
		}
		workloads, err = client.PrivateNetworks.GetSubnetWorkloads(ctx, network.ID, subnet.ID)
		if err != nil {
			t.Fatalf("failed to list subnet workloads: %v", err)
		}
		if len(workloads) != 0 {
			t.Errorf("expected no workloads on a fresh subnet, got %d", len(workloads))
		}
	})

	// A second network, created without a mode. It is used by
	// create_network_default_mode and subnet_workloads_wrong_network.
	other, err := client.PrivateNetworks.Create(ctx, verda.CreatePrivateNetworkRequest{
		Name: generateRandomName("sdk-test-net"),
	})
	if err != nil {
		t.Fatalf("failed to create network without a mode: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := client.PrivateNetworks.Delete(cleanupCtx, other.ID); err != nil {
			t.Logf("⚠️  Failed to delete private network %s: %v", other.ID, err)
		}
	})

	t.Run("create_network_default_mode", func(t *testing.T) {
		if other.Mode != verda.NetworkModeCustom {
			t.Errorf("expected mode %q when omitted, got %q", verda.NetworkModeCustom, other.Mode)
		}
	})

	t.Run("subnet_workloads_wrong_network", func(t *testing.T) {
		// A subnet of one network is not found under another network
		_, err := client.PrivateNetworks.GetSubnetWorkloads(ctx, other.ID, subnet.ID)
		requireStatus(t, err, http.StatusNotFound)
	})

	t.Run("location_private_network_fields", func(t *testing.T) {
		locations, err := client.Locations.Get(ctx)
		if err != nil {
			t.Fatalf("failed to get locations: %v", err)
		}
		for _, loc := range locations {
			if loc.Code != locationCode {
				continue
			}
			if !loc.IsPrivateNetworksEnabled {
				t.Errorf("expected is_private_networks_enabled=true for %s", locationCode)
			}
			if loc.PrivateNetworkAutoSubnet == nil {
				t.Fatalf("expected private_network_auto_subnet for %s", locationCode)
			}
			prefix, err := netip.ParsePrefix(*loc.PrivateNetworkAutoSubnet)
			if err != nil || !prefix.Addr().Is4() {
				t.Errorf("private_network_auto_subnet %q is not an IPv4 prefix (err: %v)", *loc.PrivateNetworkAutoSubnet, err)
			}
			return
		}
		t.Fatalf("location %s not in the list", locationCode)
	})

	t.Run("get_missing_network", func(t *testing.T) {
		_, err := client.PrivateNetworks.GetByID(ctx, "pn_sdkmissing")
		requireStatus(t, err, http.StatusNotFound)
	})

	t.Run("delete_subnet", func(t *testing.T) {
		if err := client.PrivateNetworks.DeleteSubnet(ctx, network.ID, subnet.ID); err != nil {
			t.Fatalf("failed to delete subnet: %v", err)
		}
	})
}

// requireStatus fails the test unless err is an *verda.APIError with the given
// HTTP status.
func requireStatus(t *testing.T, err error, status int) {
	t.Helper()
	var apiErr *verda.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
		t.Errorf("expected an API error with status %d, got %v", status, err)
	}
}
