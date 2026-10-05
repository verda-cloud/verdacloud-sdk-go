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
	"log"
	"net/http"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestInstanceService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get all instances", func(t *testing.T) {
		ctx := context.Background()
		instances, err := client.Instances.Get(ctx, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("expected 1 instance, got %d", len(instances))
		}

		instance := instances[0]
		if instance.ID != "inst_123" {
			t.Errorf("expected instance ID 'inst_123', got '%s'", instance.ID)
		}

		if instance.InstanceType != "1V100.6V" {
			t.Errorf("expected instance type '1V100.6V', got '%s'", instance.InstanceType)
		}

		if instance.Status != StatusRunning {
			t.Errorf("expected status '%s', got '%s'", StatusRunning, instance.Status)
		}
	})

	t.Run("get instances with status filter", func(t *testing.T) {
		ctx := context.Background()
		instances, err := client.Instances.Get(ctx, StatusRunning)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("expected 1 instance, got %d", len(instances))
		}
	})
}

func TestInstanceService_List(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("sends status and repeated tag filters", func(t *testing.T) {
		mockServer.SetHandler(http.MethodGet, "/instances", func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			if got := query.Get("status"); got != StatusRunning {
				t.Errorf("expected status %q, got %q", StatusRunning, got)
			}
			tags := query["tag"]
			if len(tags) != 2 || tags[0] != "environment=production" || tags[1] != "benchmark" {
				t.Errorf("expected tags [environment=production benchmark], got %v", tags)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]Instance{{ID: "inst_123"}})
		})

		instances, err := client.Instances.List(context.Background(), &ListInstancesOptions{
			Status: StatusRunning,
			Tags: []TagFilter{
				{Key: "environment", Value: "production"},
				{Key: "benchmark"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(instances) != 1 || instances[0].ID != "inst_123" {
			t.Errorf("unexpected instances: %+v", instances)
		}
	})

	t.Run("sends no query without filters", func(t *testing.T) {
		mockServer.SetHandler(http.MethodGet, "/instances", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery != "" {
				t.Errorf("expected empty query, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]Instance{})
		})

		if _, err := client.Instances.List(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error with nil options: %v", err)
		}
		if _, err := client.Instances.List(context.Background(), &ListInstancesOptions{}); err != nil {
			t.Fatalf("unexpected error with empty options: %v", err)
		}
	})

	t.Run("rejects invalid tag filter before request", func(t *testing.T) {
		mockServer.SetHandler(http.MethodGet, "/instances", func(w http.ResponseWriter, r *http.Request) {
			t.Error("request should not be sent for invalid filter")
		})

		_, err := client.Instances.List(context.Background(), &ListInstancesOptions{
			Tags: []TagFilter{{Key: "a=b"}},
		})
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
	})
}

func TestInstanceService_GetByID(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get instance by ID", func(t *testing.T) {
		ctx := context.Background()
		instanceID := "test_instance_123"
		instance, err := client.Instances.GetByID(ctx, instanceID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance == nil {
			t.Fatal("expected instance, got nil")
		}

		if instance.ID != instanceID {
			t.Errorf("expected instance ID '%s', got '%s'", instanceID, instance.ID)
		}

		if instance.InstanceType != "1V100.6V" {
			t.Errorf("expected instance type '1V100.6V', got '%s'", instance.InstanceType)
		}
	})
}

func TestInstanceService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("create instance with minimal config", func(t *testing.T) {
		req := CreateInstanceRequest{
			InstanceType: "1V100.6V",
			Image:        "ubuntu-24.04-cuda-12.8-open-docker",
			Hostname:     "test-instance",
			Description:  "Test instance",
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance == nil {
			t.Fatal("expected instance, got nil")
		}

		if instance.ID != "inst_new_123" {
			t.Errorf("expected instance ID 'inst_new_123', got '%s'", instance.ID)
		}

		if instance.InstanceType != req.InstanceType {
			t.Errorf("expected instance type '%s', got '%s'", req.InstanceType, instance.InstanceType)
		}

		if instance.Hostname != req.Hostname {
			t.Errorf("expected hostname '%s', got '%s'", req.Hostname, instance.Hostname)
		}

		// Should set default location
		if instance.Location != LocationFIN03 {
			t.Errorf("expected location '%s', got '%s'", LocationFIN03, instance.Location)
		}
	})

	t.Run("create instance with full config", func(t *testing.T) {
		req := CreateInstanceRequest{
			InstanceType:    "8V100.48V",
			Image:           "custom-image",
			Hostname:        "ml-server",
			Description:     "ML training server",
			SSHKeyIDs:       []string{"key_123", "key_456"},
			LocationCode:    "US-01",
			Contract:        "PAY_AS_YOU_GO",
			Pricing:         "FIXED_PRICE",
			StartupScriptID: stringPtr("script_123"),
			Volumes: []VolumeCreateRequest{
				{Size: 500, Type: VolumeTypeNVMe, Name: "data"},
			},
			ExistingVolumes: []string{"vol_123"},
			OSVolume:        &OSVolumeCreateRequest{Size: 100, Name: "os-vol"},
			IsSpot:          true,
			Coupon:          stringPtr("DISCOUNT20"),
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance == nil {
			t.Fatal("expected instance, got nil")
		}

		if instance.Location != "US-01" {
			t.Errorf("expected location 'US-01', got '%s'", instance.Location)
		}

		if len(instance.SSHKeyIDs) != 2 {
			t.Errorf("expected 2 SSH keys, got %d", len(instance.SSHKeyIDs))
		}
	})

	t.Run("create instance on private network", func(t *testing.T) {
		privateIP := IPAllocAuto
		publicIP := IPAllocAuto
		req := CreateInstanceRequest{
			InstanceType: "CPU.4V.16G",
			Image:        "ubuntu-24.04",
			Hostname:     "vpc-instance",
			Description:  "instance on a private network",
			Network:      "pn_123",
			Subnet:       "sn_123",
			PrivateIP:    &privateIP,
			PublicIP:     &publicIP,
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance == nil {
			t.Fatal("expected instance, got nil")
		}

		if instance.PrivateIP == nil || *instance.PrivateIP != "10.0.0.5" {
			t.Errorf("expected assigned private IP 10.0.0.5, got %v", instance.PrivateIP)
		}

		if instance.PrivateNetworkID == nil || *instance.PrivateNetworkID != "pn_123" {
			t.Errorf("expected private network pn_123, got %v", instance.PrivateNetworkID)
		}

		if instance.SubnetID == nil || *instance.SubnetID != "sn_123" {
			t.Errorf("expected subnet sn_123, got %v", instance.SubnetID)
		}
	})

	t.Run("create instance with literal IPs", func(t *testing.T) {
		privateIP, err := IPAllocIPv4("10.0.0.7")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		publicIP, err := IPAllocIPv4("203.0.113.10")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := CreateInstanceRequest{
			InstanceType: "CPU.4V.16G",
			Image:        "ubuntu-24.04",
			Hostname:     "vpc-static",
			Description:  "instance with literal IPs",
			PrivateIP:    &privateIP,
			PublicIP:     &publicIP,
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance.PrivateIP == nil || *instance.PrivateIP != "10.0.0.7" {
			t.Errorf("expected private IP 10.0.0.7, got %v", instance.PrivateIP)
		}
	})

	t.Run("create instance without public IP", func(t *testing.T) {
		privateIP := IPAllocAuto
		publicIP := IPAllocNone
		req := CreateInstanceRequest{
			InstanceType: "CPU.4V.16G",
			Image:        "ubuntu-24.04",
			Hostname:     "vpc-private-only",
			Description:  "instance without a public address",
			PrivateIP:    &privateIP,
			PublicIP:     &publicIP,
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if instance.IP != nil {
			t.Errorf("expected no public IP, got %v", *instance.IP)
		}

		if instance.PrivateIP == nil {
			t.Error("expected assigned private IP")
		}
	})

	t.Run("network field validation", func(t *testing.T) {
		privateIP := IPAllocAuto
		publicIP := IPAllocNone
		bogusIP := IPAllocation("999.1.1.1")

		base := CreateInstanceRequest{
			InstanceType: "CPU.4V.16G",
			Image:        "ubuntu-24.04",
			Hostname:     "vpc-invalid",
			Description:  "invalid network config",
		}

		ctx := context.Background()

		onlyPrivate := base
		onlyPrivate.PrivateIP = &privateIP
		if _, err := client.Instances.Create(ctx, onlyPrivate); err == nil {
			t.Error("expected error when only private_ip is set")
		}

		onlyPublic := base
		onlyPublic.PublicIP = &privateIP
		if _, err := client.Instances.Create(ctx, onlyPublic); err == nil {
			t.Error("expected error when only public_ip is set")
		}

		bothNone := base
		noneIP := IPAllocNone
		bothNone.PrivateIP = &noneIP
		bothNone.PublicIP = &publicIP
		if _, err := client.Instances.Create(ctx, bothNone); err == nil {
			t.Error("expected error when both IPs are \"none\"")
		}

		badFormat := base
		badFormat.PrivateIP = &bogusIP
		badFormat.PublicIP = &privateIP
		if _, err := client.Instances.Create(ctx, badFormat); err == nil {
			t.Error("expected error for malformed IP allocation")
		}
	})
}

func TestInstanceService_CreateSpotWithDiscontinuePolicy(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("create spot instance with on_spot_discontinue on os_volume", func(t *testing.T) {
		req := CreateInstanceRequest{
			InstanceType: "CPU.4V.16G",
			Image:        "ubuntu-24.04",
			Hostname:     "test-spot-instance",
			Description:  "test spot instance",
			SSHKeyIDs:    []string{"key_123"},
			LocationCode: "FIN-03",
			IsSpot:       true,
			OSVolume: &OSVolumeCreateRequest{
				Name:              "test-os-volume",
				Size:              55,
				OnSpotDiscontinue: SpotDiscontinueDeletePermanent,
			},
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if instance == nil {
			t.Fatal("expected instance, got nil")
		}
	})

	t.Run("create spot instance with on_spot_discontinue on additional volumes", func(t *testing.T) {
		req := CreateInstanceRequest{
			InstanceType: "CPU.4V.16G",
			Image:        "ubuntu-24.04",
			Hostname:     "test-spot-volumes",
			Description:  "test spot volumes",
			SSHKeyIDs:    []string{"key_123"},
			LocationCode: "FIN-03",
			IsSpot:       true,
			OSVolume: &OSVolumeCreateRequest{
				Name:              "test-os-volume",
				Size:              55,
				OnSpotDiscontinue: SpotDiscontinueKeepDetached,
			},
			Volumes: []VolumeCreateRequest{
				{
					Size:              500,
					Type:              VolumeTypeNVMe,
					Name:              "data-vol",
					OnSpotDiscontinue: SpotDiscontinueMoveToTrash,
				},
			},
		}

		ctx := context.Background()
		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if instance == nil {
			t.Fatal("expected instance, got nil")
		}
	})
}

func TestInstanceService_Action(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("202 single instance returns results", func(t *testing.T) {
		ctx := context.Background()
		results, err := client.Instances.Action(ctx, InstanceActionRequest{
			Action: ActionShutdown,
			ID:     []string{"inst_123"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].InstanceID != "inst_123" {
			t.Errorf("expected instanceId 'inst_123', got '%s'", results[0].InstanceID)
		}
		if results[0].Status != "success" {
			t.Errorf("expected status 'success', got '%s'", results[0].Status)
		}
	})

	t.Run("202 multiple instances returns results", func(t *testing.T) {
		ctx := context.Background()
		results, err := client.Instances.Action(ctx, InstanceActionRequest{
			Action: ActionShutdown,
			ID:     []string{"inst_123", "inst_456"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("action delete with volumes", func(t *testing.T) {
		ctx := context.Background()
		results, err := client.Instances.Action(ctx, InstanceActionRequest{
			Action:    ActionDelete,
			ID:        []string{"inst_123"},
			VolumeIDs: []string{"vol_123"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("action delete with empty volume_ids (no volumes deleted)", func(t *testing.T) {
		ctx := context.Background()
		results, err := client.Instances.Action(ctx, InstanceActionRequest{
			Action:    ActionDelete,
			ID:        []string{"inst_123"},
			VolumeIDs: []string{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("action delete with nil volume_ids (API default)", func(t *testing.T) {
		ctx := context.Background()
		results, err := client.Instances.Action(ctx, InstanceActionRequest{
			Action: ActionDelete,
			ID:     []string{"inst_123"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("action delete with volumes permanently", func(t *testing.T) {
		ctx := context.Background()
		results, err := client.Instances.Action(ctx, InstanceActionRequest{
			Action:            ActionDelete,
			ID:                []string{"inst_123"},
			VolumeIDs:         []string{"vol_123"},
			DeletePermanently: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("action discontinue with volumes permanently", func(t *testing.T) {
		ctx := context.Background()
		results, err := client.Instances.Action(ctx, InstanceActionRequest{
			Action:            ActionDiscontinue,
			ID:                []string{"inst_123"},
			VolumeIDs:         []string{"vol_123"},
			DeletePermanently: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

}

func TestInstanceService_Action_204_AlreadyInState(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	mockServer.SetHandler("PUT", "/instances", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	client := NewTestClient(mockServer)
	ctx := context.Background()

	results, err := client.Instances.Action(ctx, InstanceActionRequest{
		Action: ActionBoot,
		ID:     []string{"inst_123"},
	})
	if err != nil {
		t.Fatalf("unexpected error for 204: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for 204, got %v", results)
	}
}

func TestInstanceService_Action_207_PartialFailure(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	mockServer.SetHandler("PUT", "/instances", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMultiStatus)
		writeTestJSON(w, []map[string]interface{}{
			{"action": "shutdown", "instanceId": "inst_123", "status": "success"},
			{"action": "shutdown", "instanceId": "inst_456", "status": "error", "error": "instance not found", "statusCode": 404},
		})
	})

	client := NewTestClient(mockServer)
	ctx := context.Background()

	results, err := client.Instances.Action(ctx, InstanceActionRequest{
		Action: ActionShutdown,
		ID:     []string{"inst_123", "inst_456"},
	})

	if err != nil {
		t.Fatalf("unexpected error for 207: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != "success" {
		t.Errorf("expected first result success, got %s", results[0].Status)
	}
	if results[1].Status != "error" {
		t.Errorf("expected second result error, got %s", results[1].Status)
	}
	if results[1].Error != "instance not found" {
		t.Errorf("expected error message 'instance not found', got '%s'", results[1].Error)
	}
	if results[1].StatusCode != 404 {
		t.Errorf("expected status code 404, got %d", results[1].StatusCode)
	}
}

func TestInstanceService_Action_400_BadRequest(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	mockServer.SetHandler("PUT", "/instances", func(w http.ResponseWriter, _ *http.Request) {
		testutil.ErrorResponse(w, http.StatusBadRequest, "action not allowed in current state")
	})

	client := NewTestClient(mockServer)
	ctx := context.Background()

	results, err := client.Instances.Action(ctx, InstanceActionRequest{
		Action: ActionBoot,
		ID:     []string{"inst_123"},
	})

	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", apiErr.StatusCode)
	}

	if results != nil {
		t.Errorf("expected nil results for 400, got %v", results)
	}
}

func TestInstanceService_Action_404_NotFound(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	mockServer.SetHandler("PUT", "/instances", func(w http.ResponseWriter, _ *http.Request) {
		testutil.ErrorResponse(w, http.StatusNotFound, "instance not found")
	})

	client := NewTestClient(mockServer)
	ctx := context.Background()

	results, err := client.Instances.Action(ctx, InstanceActionRequest{
		Action: ActionDelete,
		ID:     []string{"inst_nonexistent"},
	})

	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}

	if results != nil {
		t.Errorf("expected nil results for 404, got %v", results)
	}
}

func TestInstanceService_ConvenienceMethods(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("boot instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Boot(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("start instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Start(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("shutdown instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Shutdown(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("delete instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Delete(ctx, []string{"inst_123"}, []string{"vol_123"}, false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("delete instance permanently", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Delete(ctx, []string{"inst_123"}, []string{"vol_123"}, true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("discontinue instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Discontinue(ctx, []string{"inst_123"}, nil, false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("discontinue instance with volumes permanently", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Discontinue(ctx, []string{"inst_123"}, []string{"vol_123"}, true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("hibernate instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Hibernate(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("configure spot instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.ConfigureSpot(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("force shutdown instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.ForceShutdown(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("delete stuck instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.DeleteStuck(ctx, []string{"vol_123"}, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("deploy instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Deploy(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("transfer instance", func(t *testing.T) {
		ctx := context.Background()
		err := client.Instances.Transfer(ctx, "inst_123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func writeTestJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("test: failed to encode JSON: %v", err)
	}
}

func stringPtr(s string) *string {
	return &s
}
