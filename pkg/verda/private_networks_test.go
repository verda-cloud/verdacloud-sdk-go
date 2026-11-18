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
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestPrivateNetworkService_Networks(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
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

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	isDefaultTrue := true

	t.Run("create subnet", func(t *testing.T) {
		// Its own network, so the stored subnet does not change the
		// fixture-based pn_123 subtests below
		subnet, err := client.PrivateNetworks.CreateSubnet(ctx, "pn_create", CreateSubnetRequest{
			LocationCode: "FIN-01",
			Name:         "workers",
			CIDR:         "10.0.0.0/24",
			IsDefault:    &isDefaultTrue,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.HasPrefix(subnet.ID, "sn_new_") {
			t.Errorf("expected a generated subnet ID, got '%s'", subnet.ID)
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

	t.Run("create subnet is_default on the wire", func(t *testing.T) {
		isFalse, isTrue := false, true
		cases := []struct {
			name      string
			isDefault *bool
			want      any // nil means the key must be absent
		}{
			{"omitted", nil, nil},
			{"explicit false", &isFalse, false},
			{"explicit true", &isTrue, true},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				srv := testutil.NewMockServer()
				defer srv.Close()

				var body map[string]any
				srv.SetHandler(http.MethodPost, "/private-networks/pn_wire/subnets", func(w http.ResponseWriter, r *http.Request) {
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode request body: %v", err)
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id":"sn_wire","cidr":"10.0.0.0/24"}`))
				})

				c := NewTestClient(t, srv)
				_, err := c.PrivateNetworks.CreateSubnet(ctx, "pn_wire", CreateSubnetRequest{
					LocationCode: "FIN-01",
					Name:         "workers",
					CIDR:         "10.0.0.0/24",
					IsDefault:    tc.isDefault,
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				got, present := body["is_default"]
				if tc.want == nil {
					if present {
						t.Errorf("expected is_default to be absent, got %v", got)
					}
					return
				}
				if !present || got != tc.want {
					t.Errorf("expected is_default %v, got %v (present=%v)", tc.want, got, present)
				}
			})
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

// TestPrivateNetworkService_SubnetDefaultRule checks that the mock server
// follows the API's is_default rule: a subnet created without is_default
// becomes the default only when its network has no default subnet in that
// location; is_default true moves the default; deleting the default does not
// move it to another subnet.
func TestPrivateNetworkService_SubnetDefaultRule(t *testing.T) {
	ctx := context.Background()
	isTrue, isFalse := true, false

	setup := func(t *testing.T) *Client {
		t.Helper()
		srv := testutil.NewMockServer()
		t.Cleanup(srv.Close)
		return NewTestClient(t, srv)
	}

	create := func(t *testing.T, c *Client, networkID, location string, isDefault *bool) *Subnet {
		t.Helper()
		subnet, err := c.PrivateNetworks.CreateSubnet(ctx, networkID, CreateSubnetRequest{
			LocationCode: location,
			Name:         "workers",
			CIDR:         "10.0.0.0/24",
			IsDefault:    isDefault,
		})
		if err != nil {
			t.Fatalf("create subnet: %v", err)
		}
		return subnet
	}

	// defaults returns the is_default flag of every subnet in the network, by ID
	defaults := func(t *testing.T, c *Client, networkID string) map[string]bool {
		t.Helper()
		subnets, err := c.PrivateNetworks.GetSubnets(ctx, networkID)
		if err != nil {
			t.Fatalf("list subnets: %v", err)
		}
		got := make(map[string]bool, len(subnets))
		for _, s := range subnets {
			got[s.ID] = s.IsDefault
		}
		return got
	}

	assertDefaults := func(t *testing.T, c *Client, networkID string, want map[string]bool) {
		t.Helper()
		got := defaults(t, c, networkID)
		if len(got) != len(want) {
			t.Fatalf("expected %d subnets in %s, got %v", len(want), networkID, got)
		}
		for id, wantDefault := range want {
			gotDefault, ok := got[id]
			if !ok {
				t.Errorf("subnet %s missing from %s: %v", id, networkID, got)
				continue
			}
			if gotDefault != wantDefault {
				t.Errorf("subnet %s: expected is_default %v, got %v", id, wantDefault, gotDefault)
			}
		}
	}

	t.Run("omitted: first in a location becomes default, second does not", func(t *testing.T) {
		c := setup(t)
		first := create(t, c, "pn_a", "FIN-01", nil)
		second := create(t, c, "pn_a", "FIN-01", nil)

		if !first.IsDefault || second.IsDefault {
			t.Errorf("expected first default and second not, got %v and %v", first.IsDefault, second.IsDefault)
		}
		assertDefaults(t, c, "pn_a", map[string]bool{first.ID: true, second.ID: false})
	})

	t.Run("omitted: becomes default when only non-default subnets exist", func(t *testing.T) {
		c := setup(t)
		first := create(t, c, "pn_a", "FIN-01", &isFalse)
		second := create(t, c, "pn_a", "FIN-01", nil)

		if first.IsDefault || !second.IsDefault {
			t.Errorf("expected only the second to be default, got %v and %v", first.IsDefault, second.IsDefault)
		}
		assertDefaults(t, c, "pn_a", map[string]bool{first.ID: false, second.ID: true})
	})

	t.Run("create with true moves the default", func(t *testing.T) {
		c := setup(t)
		first := create(t, c, "pn_a", "FIN-01", nil)
		second := create(t, c, "pn_a", "FIN-01", &isTrue)

		if !second.IsDefault {
			t.Error("expected the new subnet to be default")
		}
		assertDefaults(t, c, "pn_a", map[string]bool{first.ID: false, second.ID: true})
	})

	t.Run("update with true moves the default", func(t *testing.T) {
		c := setup(t)
		first := create(t, c, "pn_a", "FIN-01", nil)
		second := create(t, c, "pn_a", "FIN-01", nil)

		updated, err := c.PrivateNetworks.UpdateSubnet(ctx, "pn_a", second.ID, UpdateSubnetRequest{IsDefault: &isTrue})
		if err != nil {
			t.Fatalf("update subnet: %v", err)
		}
		if !updated.IsDefault {
			t.Error("expected the updated subnet to be default")
		}
		assertDefaults(t, c, "pn_a", map[string]bool{first.ID: false, second.ID: true})
	})

	t.Run("rename keeps the default flag", func(t *testing.T) {
		c := setup(t)
		first := create(t, c, "pn_a", "FIN-01", nil)
		second := create(t, c, "pn_a", "FIN-01", nil)

		name := "renamed"
		for _, s := range []*Subnet{first, second} {
			updated, err := c.PrivateNetworks.UpdateSubnet(ctx, "pn_a", s.ID, UpdateSubnetRequest{Name: &name})
			if err != nil {
				t.Fatalf("update subnet: %v", err)
			}
			if updated.Name != name || updated.IsDefault != s.IsDefault {
				t.Errorf("subnet %s: expected name %q and is_default %v, got %q and %v",
					s.ID, name, s.IsDefault, updated.Name, updated.IsDefault)
			}
		}
		assertDefaults(t, c, "pn_a", map[string]bool{first.ID: true, second.ID: false})
	})

	t.Run("deleting the default does not promote another subnet", func(t *testing.T) {
		c := setup(t)
		first := create(t, c, "pn_a", "FIN-01", nil)
		second := create(t, c, "pn_a", "FIN-01", nil)

		if err := c.PrivateNetworks.DeleteSubnet(ctx, "pn_a", first.ID); err != nil {
			t.Fatalf("delete subnet: %v", err)
		}
		assertDefaults(t, c, "pn_a", map[string]bool{second.ID: false})

		third := create(t, c, "pn_a", "FIN-01", nil)
		if !third.IsDefault {
			t.Error("expected a subnet created after the default was deleted to become default")
		}
		assertDefaults(t, c, "pn_a", map[string]bool{second.ID: false, third.ID: true})
	})

	t.Run("unsetting the default leaves the location without one", func(t *testing.T) {
		c := setup(t)
		first := create(t, c, "pn_a", "FIN-01", nil)

		if _, err := c.PrivateNetworks.UpdateSubnet(ctx, "pn_a", first.ID, UpdateSubnetRequest{IsDefault: &isFalse}); err != nil {
			t.Fatalf("update subnet: %v", err)
		}
		second := create(t, c, "pn_a", "FIN-01", nil)
		if !second.IsDefault {
			t.Error("expected a subnet created after the default was unset to become default")
		}
		assertDefaults(t, c, "pn_a", map[string]bool{first.ID: false, second.ID: true})
	})

	t.Run("the rule is per network and location", func(t *testing.T) {
		c := setup(t)
		fin01 := create(t, c, "pn_a", "FIN-01", nil)
		fin03 := create(t, c, "pn_a", "FIN-03", nil)
		other := create(t, c, "pn_b", "FIN-01", nil)

		if !fin01.IsDefault || !fin03.IsDefault || !other.IsDefault {
			t.Errorf("expected each first subnet to be default, got %v, %v, %v", fin01.IsDefault, fin03.IsDefault, other.IsDefault)
		}

		moved := create(t, c, "pn_a", "FIN-01", &isTrue)
		assertDefaults(t, c, "pn_a", map[string]bool{fin01.ID: false, fin03.ID: true, moved.ID: true})
		assertDefaults(t, c, "pn_b", map[string]bool{other.ID: true})
	})

	t.Run("a deleted subnet does not come back", func(t *testing.T) {
		c := setup(t)
		subnet := create(t, c, "pn_a", "FIN-01", nil)

		if err := c.PrivateNetworks.DeleteSubnet(ctx, "pn_a", subnet.ID); err != nil {
			t.Fatalf("delete subnet: %v", err)
		}
		assertDefaults(t, c, "pn_a", map[string]bool{})

		_, err := c.PrivateNetworks.GetSubnetByID(ctx, "pn_a", subnet.ID)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected a 404 APIError, got %v", err)
		}
	})
}

func TestPrivateNetworkService_Workloads(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	assertWorkloadFixtures := func(t *testing.T, workloads []PrivateNetworkWorkload, wantSubnet string) {
		t.Helper()
		if len(workloads) != 2 {
			t.Fatalf("expected 2 workloads, got %d", len(workloads))
		}

		full := workloads[0]
		if full.ID != "inst_wl_full" || full.SubnetID != wantSubnet || full.Hostname == "" || full.Status != StatusRunning {
			t.Errorf("unexpected workload: %+v", full)
		}
		if full.Location.Code != "FIN-01" || full.PrivateIP != "10.0.0.5" || full.CreatedAt.IsZero() {
			t.Errorf("unexpected location, private IP or created_at: %+v", full)
		}
		if full.PublicIP == nil || full.CreatedByUserID == nil {
			t.Error("expected public_ip and created_by_user_id to be set")
		}
		if full.Model.Code != "B200" || full.Model.Name != "NVIDIA B200" || full.Model.Category != "gpu" ||
			full.Model.Manufacturer != "NVIDIA" || full.Model.ClusterModelCode == nil || *full.Model.ClusterModelCode != "B200" {
			t.Errorf("unexpected model: %+v", full.Model)
		}
		if full.Template.InstanceType != "1B200.30V" || full.Template.CPUCores != 30 || full.Template.GPUNumber != 1 ||
			full.Template.GPUVRAMGB != 180 || full.Template.RAMGB != 184 ||
			full.Template.CPUModel == nil || *full.Template.CPUModel != "AMD EPYC 9654" {
			t.Errorf("unexpected template: %+v", full.Template)
		}
		if full.PublicIP != nil && *full.PublicIP == "" {
			t.Error("expected a non-empty public_ip")
		}
		if full.CreatedByUserID != nil && *full.CreatedByUserID != "7dfc2631-0158-4f74-93f6-6b46dcb90fef" {
			t.Errorf("unexpected created_by_user_id %q", *full.CreatedByUserID)
		}

		nulls := workloads[1]
		if nulls.PublicIP != nil || nulls.CreatedByUserID != nil || nulls.Model.ClusterModelCode != nil || nulls.Template.CPUModel != nil {
			t.Errorf("expected null fields to decode as nil: %+v", nulls)
		}
	}

	t.Run("network workloads", func(t *testing.T) {
		workloads, err := client.PrivateNetworks.GetWorkloads(ctx, "pn_123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertWorkloadFixtures(t, workloads, "sn_123")
	})

	t.Run("subnet workloads", func(t *testing.T) {
		workloads, err := client.PrivateNetworks.GetSubnetWorkloads(ctx, "pn_123", "sn_other")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertWorkloadFixtures(t, workloads, "sn_other")
	})

	t.Run("empty list", func(t *testing.T) {
		workloads, err := client.PrivateNetworks.GetWorkloads(ctx, "pn_empty")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if workloads == nil || len(workloads) != 0 {
			t.Errorf("expected an empty, non-nil slice, got %#v", workloads)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := client.PrivateNetworks.GetWorkloads(ctx, "pn_missing")
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("GetWorkloads: expected 404 APIError, got %v", err)
		}

		_, err = client.PrivateNetworks.GetSubnetWorkloads(ctx, "pn_missing", "sn_123")
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("GetSubnetWorkloads: expected 404 APIError, got %v", err)
		}
	})
}

func TestPrivateNetworkService_Routes(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
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
