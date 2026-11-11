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
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

const testClusterJSON = `{
	"name": "my-cluster",
	"kubernetes_version": "1.31.1",
	"location": "FIN-03",
	"network": "net_123",
	"subnet": "sub_123",
	"pod_cidr": "10.0.0.0/21",
	"service_cidr": "10.1.0.0/16",
	"dns_suffix": "cluster.local",
	"phase": "running",
	"external_endpoint": "https://k8s.example.com:6443",
	"conditions": [
		{
			"type": "Ready",
			"status": "True",
			"observedGeneration": 2,
			"lastTransitionTime": "2026-10-30T14:00:00Z",
			"reason": "AllComponentsReady",
			"message": "all components are ready"
		}
	],
	"created_at": "2026-10-30T14:00:00Z"
}`

func TestManagedKubernetesService_List(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/mk8sclusters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "2")
		if _, err := w.Write([]byte("[" + testClusterJSON + `,{
			"name": "other-cluster",
			"kubernetes_version": "1.31.1",
			"location": "FIN-01",
			"network": "net_456",
			"subnet": "sub_456",
			"pod_cidr": "10.2.0.0/21",
			"service_cidr": "10.3.0.0/16",
			"phase": "provisioning",
			"created_at": "2026-10-31T09:00:00Z"
		}]`)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("list clusters", func(t *testing.T) {
		clusters, err := client.ManagedKubernetes.List(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(clusters) != 2 {
			t.Fatalf("expected 2 clusters, got %d", len(clusters))
		}

		c := clusters[0]
		if c.Name != "my-cluster" || c.KubernetesVersion != "1.31.1" || c.NetworkID != "net_123" || c.SubnetID != "sub_123" {
			t.Errorf("unexpected cluster: %+v", c)
		}
		if c.Phase != ManagedKubernetesClusterPhaseRunning {
			t.Errorf("expected phase '%s', got '%s'", ManagedKubernetesClusterPhaseRunning, c.Phase)
		}

		if len(c.Conditions) != 1 {
			t.Fatalf("expected 1 condition, got %d", len(c.Conditions))
		}
		cond := c.Conditions[0]
		if cond.Type != "Ready" || cond.Status != "True" || cond.ObservedGeneration != 2 || cond.Reason != "AllComponentsReady" {
			t.Errorf("unexpected condition: %+v", cond)
		}
		if cond.LastTransitionTime.IsZero() {
			t.Error("expected lastTransitionTime to be parsed")
		}

		if clusters[1].DNSSuffix != "" || clusters[1].ExternalEndpoint != "" || clusters[1].Conditions != nil {
			t.Errorf("expected omitted optional fields to stay zero, got %+v", clusters[1])
		}
	})
}

func TestManagedKubernetesService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/my-cluster", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(testClusterJSON)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("get cluster", func(t *testing.T) {
		cluster, err := client.ManagedKubernetes.Get(context.Background(), "FIN-03", "my-cluster")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cluster.Name != "my-cluster" || cluster.Location != "FIN-03" {
			t.Errorf("unexpected cluster: %+v", cluster)
		}
		if cluster.DNSSuffix != "cluster.local" || cluster.ExternalEndpoint != "https://k8s.example.com:6443" {
			t.Errorf("unexpected cluster: %+v", cluster)
		}
		if cluster.CreatedAt.IsZero() {
			t.Error("expected created_at to be parsed")
		}
	})
}

func TestManagedKubernetesService_GetEnabledLocations(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/mk8sclusters/enabled_locations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"locations": ["FIN-01", "FIN-03"]}`)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("get enabled locations", func(t *testing.T) {
		locations, err := client.ManagedKubernetes.GetEnabledLocations(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(locations.Locations) != 2 || locations.Locations[0] != "FIN-01" || locations.Locations[1] != "FIN-03" {
			t.Errorf("unexpected locations: %+v", locations)
		}
	})
}

func TestManagedKubernetesService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodPost, "/mk8sclusters", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("mock JSON operation failed: %v", err)
			return
		}
		for _, field := range []string{"name", "kubernetes_version", "pod_cidr", "service_cidr", "location", "network", "subnet"} {
			if body[field] == nil {
				t.Errorf("expected request field %q to be sent", field)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(testClusterJSON)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	validReq := CreateManagedKubernetesClusterRequest{
		Name:              "my-cluster",
		KubernetesVersion: "1.31.1",
		PodCIDR:           "10.0.0.0/21",
		ServiceCIDR:       "10.1.0.0/16",
		Location:          "FIN-03",
		NetworkID:         "net_123",
		SubnetID:          "sub_123",
	}

	t.Run("create cluster", func(t *testing.T) {
		cluster, err := client.ManagedKubernetes.Create(context.Background(), validReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cluster.Name != "my-cluster" || cluster.Phase != ManagedKubernetesClusterPhaseRunning {
			t.Errorf("unexpected cluster: %+v", cluster)
		}
	})

	t.Run("validation failures", func(t *testing.T) {
		cases := map[string]CreateManagedKubernetesClusterRequest{
			"empty": {},
			"invalid name": func() CreateManagedKubernetesClusterRequest {
				r := validReq
				r.Name = "My_Cluster"
				return r
			}(),
			"name too long": func() CreateManagedKubernetesClusterRequest {
				r := validReq
				r.Name = strings.Repeat("a", managedKubernetesNameMaxLength+1)
				return r
			}(),
			"pod cidr too small": func() CreateManagedKubernetesClusterRequest {
				r := validReq
				r.PodCIDR = "10.0.0.0/22"
				return r
			}(),
			"pod cidr invalid": func() CreateManagedKubernetesClusterRequest {
				r := validReq
				r.PodCIDR = "not-a-cidr"
				return r
			}(),
			"overlapping cidrs": func() CreateManagedKubernetesClusterRequest {
				r := validReq
				r.ServiceCIDR = "10.0.0.0/16"
				return r
			}(),
		}

		for name, req := range cases {
			cluster, err := client.ManagedKubernetes.Create(context.Background(), req)
			if err == nil {
				t.Errorf("%s: expected validation error, got nil", name)
			}
			if cluster != nil {
				t.Errorf("%s: expected nil cluster on validation error, got %+v", name, cluster)
			}
		}
	})
}

func TestManagedKubernetesService_Update(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodPatch, "/mk8sclusters/FIN-03/my-cluster", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("mock JSON operation failed: %v", err)
			return
		}
		if len(body) != 1 || body["kubernetes_version"] != "1.32.0" {
			t.Errorf("expected only kubernetes_version in update body, got %v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(testClusterJSON)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("update kubernetes version", func(t *testing.T) {
		cluster, err := client.ManagedKubernetes.Update(context.Background(), "FIN-03", "my-cluster", UpdateManagedKubernetesClusterRequest{
			KubernetesVersion: "1.32.0",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cluster == nil {
			t.Fatal("expected cluster, got nil")
		}
	})

	t.Run("empty version rejected", func(t *testing.T) {
		_, err := client.ManagedKubernetes.Update(context.Background(), "FIN-03", "my-cluster", UpdateManagedKubernetesClusterRequest{})
		if err == nil {
			t.Error("expected validation error, got nil")
		}
	})
}

func TestManagedKubernetesService_Delete(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodDelete, "/mk8sclusters/FIN-03/my-cluster", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	t.Run("delete cluster", func(t *testing.T) {
		if err := client.ManagedKubernetes.Delete(context.Background(), "FIN-03", "my-cluster"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

const testPoolJSON = `{
	"name": "cpu-workers",
	"replicas": 3,
	"ready_replicas": 2,
	"instance_type": "CPU.4V.16",
	"ssh_key_ids": ["key_1", "key_2"],
	"state": "degraded",
	"errors": [
		{
			"reason": "insufficient_capacity",
			"message": "no CPU.4V.16 available in FIN-03",
			"since": "2026-10-30T14:05:00Z"
		}
	],
	"created_at": "2026-10-30T14:00:00Z"
}`

func TestManagedKubernetesService_ListNodePools(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/my-cluster/pools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("[" + testPoolJSON + `,{
			"name": "gpu-workers",
			"replicas": 0,
			"ready_replicas": 0,
			"instance_type": "GPU.1V.24",
			"state": "active",
			"created_at": "2026-10-31T09:00:00Z"
		}]`)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("list pools", func(t *testing.T) {
		pools, err := client.ManagedKubernetes.ListNodePools(context.Background(), "FIN-03", "my-cluster")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(pools) != 2 {
			t.Fatalf("expected 2 pools, got %d", len(pools))
		}

		p := pools[0]
		if p.Name != "cpu-workers" || p.Replicas != 3 || p.ReadyReplicas != 2 || p.InstanceType != "CPU.4V.16" {
			t.Errorf("unexpected pool: %+v", p)
		}
		if p.State != ManagedKubernetesNodePoolStateDegraded {
			t.Errorf("expected state '%s', got '%s'", ManagedKubernetesNodePoolStateDegraded, p.State)
		}

		if len(p.Errors) != 1 {
			t.Fatalf("expected 1 pool error, got %d", len(p.Errors))
		}
		poolErr := p.Errors[0]
		if poolErr.Reason != ManagedKubernetesNodePoolErrorReasonInsufficientCapacity || poolErr.Since.IsZero() {
			t.Errorf("unexpected pool error: %+v", poolErr)
		}

		if pools[1].Errors != nil || pools[1].SSHKeyIDs != nil {
			t.Errorf("expected omitted optional fields to stay zero, got %+v", pools[1])
		}
	})
}

func TestManagedKubernetesService_GetNodePool(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/my-cluster/pools/cpu-workers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(testPoolJSON)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("get pool", func(t *testing.T) {
		pool, err := client.ManagedKubernetes.GetNodePool(context.Background(), "FIN-03", "my-cluster", "cpu-workers")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pool.Name != "cpu-workers" || len(pool.SSHKeyIDs) != 2 || pool.SSHKeyIDs[0] != "key_1" {
			t.Errorf("unexpected pool: %+v", pool)
		}
		if pool.CreatedAt.IsZero() {
			t.Error("expected created_at to be parsed")
		}
	})
}

func TestManagedKubernetesService_CreateNodePool(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodPost, "/mk8sclusters/FIN-03/my-cluster/pools", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("mock JSON operation failed: %v", err)
			return
		}
		if _, sent := body["replicas"]; sent {
			t.Errorf("replicas must be omitted when nil, got %v", body)
		}
		for _, field := range []string{"name", "instance_type", "ssh_key_ids"} {
			if body[field] == nil {
				t.Errorf("expected request field %q to be sent", field)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(testPoolJSON)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	validReq := CreateManagedKubernetesNodePoolRequest{
		Name:         "cpu-workers",
		InstanceType: "CPU.4V.16",
		SSHKeyIDs:    []string{"key_1"},
	}

	t.Run("create pool", func(t *testing.T) {
		pool, err := client.ManagedKubernetes.CreateNodePool(context.Background(), "FIN-03", "my-cluster", validReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pool.Name != "cpu-workers" || pool.State != ManagedKubernetesNodePoolStateDegraded {
			t.Errorf("unexpected pool: %+v", pool)
		}
	})

	t.Run("validation failures", func(t *testing.T) {
		negative := -1

		cases := map[string]CreateManagedKubernetesNodePoolRequest{
			"empty":            {},
			"missing ssh keys": {Name: "cpu-workers", InstanceType: "CPU.4V.16"},
			"empty ssh keys":   {Name: "cpu-workers", InstanceType: "CPU.4V.16", SSHKeyIDs: []string{}},
			"invalid name": func() CreateManagedKubernetesNodePoolRequest {
				r := validReq
				r.Name = "cpu_workers"
				return r
			}(),
			"name too long": func() CreateManagedKubernetesNodePoolRequest {
				r := validReq
				r.Name = strings.Repeat("a", managedKubernetesNameMaxLength+1)
				return r
			}(),
			"negative replicas": func() CreateManagedKubernetesNodePoolRequest {
				r := validReq
				r.Replicas = &negative
				return r
			}(),
		}

		for name, req := range cases {
			pool, err := client.ManagedKubernetes.CreateNodePool(context.Background(), "FIN-03", "my-cluster", req)
			if err == nil {
				t.Errorf("%s: expected validation error, got nil", name)
			}
			if pool != nil {
				t.Errorf("%s: expected nil pool on validation error, got %+v", name, pool)
			}
		}
	})
}

func TestManagedKubernetesService_UpdateNodePool(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodPatch, "/mk8sclusters/FIN-03/my-cluster/pools/cpu-workers", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("mock JSON operation failed: %v", err)
			return
		}
		if len(body) != 1 || body["replicas"] != float64(0) {
			t.Errorf("expected only replicas: 0 in update body, got %v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(testPoolJSON)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("scale to zero", func(t *testing.T) {
		zero := 0
		pool, err := client.ManagedKubernetes.UpdateNodePool(context.Background(), "FIN-03", "my-cluster", "cpu-workers", UpdateManagedKubernetesNodePoolRequest{
			Replicas: &zero,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pool == nil {
			t.Fatal("expected pool, got nil")
		}
	})

	t.Run("nil replicas rejected", func(t *testing.T) {
		_, err := client.ManagedKubernetes.UpdateNodePool(context.Background(), "FIN-03", "my-cluster", "cpu-workers", UpdateManagedKubernetesNodePoolRequest{})
		if err == nil {
			t.Error("expected validation error, got nil")
		}
	})
}

func TestManagedKubernetesService_DeleteNodePool(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodDelete, "/mk8sclusters/FIN-03/my-cluster/pools/cpu-workers", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	t.Run("delete pool", func(t *testing.T) {
		if err := client.ManagedKubernetes.DeleteNodePool(context.Background(), "FIN-03", "my-cluster", "cpu-workers"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

const testKubeconfigReadyJSON = `{
	"id": "my-cluster-kc-def34",
	"display_name": "alice-laptop",
	"common_name": "alice",
	"groups": ["system:masters"],
	"status": "ready",
	"not_after": "2027-01-28T12:00:00Z",
	"kubeconfig": "apiVersion: v1\nclusters:\n- cluster:\n    server: https://k8s.example.com:6443\n  name: my-cluster\n"
}`

func TestManagedKubernetesService_ListKubeconfigs(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/my-cluster/kubeconfigs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`[
			{
				"id": "my-cluster-kc-abc12",
				"common_name": "bob",
				"status": "pending"
			},
			{
				"id": "my-cluster-kc-def34",
				"display_name": "alice-laptop",
				"common_name": "alice",
				"groups": ["system:masters"],
				"status": "ready",
				"not_after": "2027-01-28T12:00:00Z"
			}
		]`)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("list kubeconfigs", func(t *testing.T) {
		kubeconfigs, err := client.ManagedKubernetes.ListKubeconfigs(context.Background(), "FIN-03", "my-cluster")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(kubeconfigs) != 2 {
			t.Fatalf("expected 2 kubeconfigs, got %d", len(kubeconfigs))
		}

		pending := kubeconfigs[0]
		if pending.ID != "my-cluster-kc-abc12" || pending.Status != ManagedKubernetesKubeconfigStatusPending {
			t.Errorf("unexpected kubeconfig: %+v", pending)
		}
		if pending.NotAfter != nil || pending.Kubeconfig != "" {
			t.Errorf("expected pending kubeconfig to have no not_after and no content, got %+v", pending)
		}

		ready := kubeconfigs[1]
		if ready.Status != ManagedKubernetesKubeconfigStatusReady || ready.NotAfter == nil {
			t.Errorf("unexpected kubeconfig: %+v", ready)
		}
		if ready.Kubeconfig != "" {
			t.Errorf("list responses must not include kubeconfig content, got %q", ready.Kubeconfig)
		}
	})
}

func TestManagedKubernetesService_GetKubeconfig(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/my-cluster/kubeconfigs/my-cluster-kc-def34", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(testKubeconfigReadyJSON)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	t.Run("get kubeconfig", func(t *testing.T) {
		kubeconfig, err := client.ManagedKubernetes.GetKubeconfig(context.Background(), "FIN-03", "my-cluster", "my-cluster-kc-def34")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if kubeconfig.Status != ManagedKubernetesKubeconfigStatusReady {
			t.Errorf("expected status '%s', got '%s'", ManagedKubernetesKubeconfigStatusReady, kubeconfig.Status)
		}
		if kubeconfig.NotAfter == nil {
			t.Error("expected not_after to be parsed")
		}
		if kubeconfig.DisplayName != "alice-laptop" || len(kubeconfig.Groups) != 1 || kubeconfig.Groups[0] != "system:masters" {
			t.Errorf("unexpected kubeconfig: %+v", kubeconfig)
		}
		if !strings.Contains(kubeconfig.Kubeconfig, "apiVersion: v1") {
			t.Errorf("expected rendered kubeconfig YAML, got %q", kubeconfig.Kubeconfig)
		}
	})
}

func TestManagedKubernetesService_CreateKubeconfig(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	strictChecks := true
	mockServer.SetHandler(http.MethodPost, "/mk8sclusters/FIN-03/my-cluster/kubeconfigs", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("mock JSON operation failed: %v", err)
			return
		}
		if body["common_name"] != "alice" {
			t.Errorf("expected common_name 'alice', got %v", body)
		}
		if strictChecks {
			if _, sent := body["groups"]; sent {
				t.Errorf("groups must be omitted when nil, got %v", body)
			}
			if body["duration"] != "72h" {
				t.Errorf("expected duration '72h', got %v", body)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{
			"id": "my-cluster-kc-abc12",
			"common_name": "alice",
			"status": "pending"
		}`)); err != nil {
			t.Errorf("write mock response: %v", err)
		}
	})

	validReq := CreateManagedKubernetesKubeconfigRequest{
		CommonName: "alice",
		Duration:   "72h",
	}

	t.Run("create kubeconfig", func(t *testing.T) {
		kubeconfig, err := client.ManagedKubernetes.CreateKubeconfig(context.Background(), "FIN-03", "my-cluster", validReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if kubeconfig.Status != ManagedKubernetesKubeconfigStatusPending || kubeconfig.Kubeconfig != "" {
			t.Errorf("expected pending kubeconfig without content, got %+v", kubeconfig)
		}
	})

	t.Run("with groups", func(t *testing.T) {
		strictChecks = false
		defer func() { strictChecks = true }()

		_, err := client.ManagedKubernetes.CreateKubeconfig(context.Background(), "FIN-03", "my-cluster", CreateManagedKubernetesKubeconfigRequest{
			CommonName:  "alice",
			Groups:      []string{"system:masters", "viewers"},
			DisplayName: "alice-laptop",
			Duration:    "1h",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("validation failures", func(t *testing.T) {
		cases := map[string]CreateManagedKubernetesKubeconfigRequest{
			"empty":            {},
			"control-plane cn": {CommonName: "system:admin"},
			"control-plane group": func() CreateManagedKubernetesKubeconfigRequest {
				r := validReq
				r.Groups = []string{"system:authenticated"}
				return r
			}(),
			"empty group": func() CreateManagedKubernetesKubeconfigRequest {
				r := validReq
				r.Groups = []string{"  "}
				return r
			}(),
			"duration too short": func() CreateManagedKubernetesKubeconfigRequest {
				r := validReq
				r.Duration = "30m"
				return r
			}(),
			"duration invalid": func() CreateManagedKubernetesKubeconfigRequest {
				r := validReq
				r.Duration = "three days"
				return r
			}(),
		}

		for name, req := range cases {
			kubeconfig, err := client.ManagedKubernetes.CreateKubeconfig(context.Background(), "FIN-03", "my-cluster", req)
			if err == nil {
				t.Errorf("%s: expected validation error, got nil", name)
			}
			if kubeconfig != nil {
				t.Errorf("%s: expected nil kubeconfig on validation error, got %+v", name, kubeconfig)
			}
		}
	})
}

func TestManagedKubernetesService_DeleteKubeconfig(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodDelete, "/mk8sclusters/FIN-03/my-cluster/kubeconfigs/my-cluster-kc-def34", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	t.Run("delete kubeconfig", func(t *testing.T) {
		if err := client.ManagedKubernetes.DeleteKubeconfig(context.Background(), "FIN-03", "my-cluster", "my-cluster-kc-def34"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestManagedKubernetesService_ErrorHandling(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	respond := func(status int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			if _, err := w.Write([]byte(body)); err != nil {
				t.Errorf("write mock response: %v", err)
			}
		}
	}

	notFound := `{"code": "not_found", "message": "cluster not found", "details": {"reason": "cluster_not_found"}}`
	mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/no-cluster", respond(http.StatusNotFound, notFound))
	mockServer.SetHandler(http.MethodDelete, "/mk8sclusters/FIN-03/no-cluster", respond(http.StatusNotFound, notFound))
	mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/my-cluster/pools/no-pool",
		respond(http.StatusNotFound, `{"code": "not_found", "message": "pool not found"}`))
	mockServer.SetHandler(http.MethodDelete, "/mk8sclusters/FIN-03/deleting-cluster",
		respond(http.StatusConflict, `{"code": "conflict", "message": "cluster is being deleted", "details": {"reason": "cluster_deleting"}}`))

	asAPIError := func(t *testing.T, err error) *APIError {
		t.Helper()
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		return apiErr
	}

	t.Run("get missing cluster", func(t *testing.T) {
		_, err := client.ManagedKubernetes.Get(context.Background(), "FIN-03", "no-cluster")
		apiErr := asAPIError(t, err)
		if apiErr.StatusCode != http.StatusNotFound || apiErr.Code != "not_found" || apiErr.Message != "cluster not found" ||
			apiErr.Details != `{"reason":"cluster_not_found"}` {
			t.Errorf("unexpected error: %+v", apiErr)
		}
	})

	t.Run("delete already deleted cluster", func(t *testing.T) {
		err := client.ManagedKubernetes.Delete(context.Background(), "FIN-03", "no-cluster")
		if apiErr := asAPIError(t, err); apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %+v", apiErr)
		}
	})

	t.Run("get missing node pool", func(t *testing.T) {
		_, err := client.ManagedKubernetes.GetNodePool(context.Background(), "FIN-03", "my-cluster", "no-pool")
		apiErr := asAPIError(t, err)
		if apiErr.StatusCode != http.StatusNotFound || apiErr.Message != "pool not found" {
			t.Errorf("unexpected error: %+v", apiErr)
		}
	})

	t.Run("conflict with reason", func(t *testing.T) {
		err := client.ManagedKubernetes.Delete(context.Background(), "FIN-03", "deleting-cluster")
		apiErr := asAPIError(t, err)
		if apiErr.StatusCode != http.StatusConflict || apiErr.Code != "conflict" || apiErr.Details != `{"reason":"cluster_deleting"}` {
			t.Errorf("unexpected error: %+v", apiErr)
		}
	})
}

func TestManagedKubernetesCreateRequests_NameLengthBoundaries(t *testing.T) {
	cluster := CreateManagedKubernetesClusterRequest{
		KubernetesVersion: "1.31.1",
		PodCIDR:           "10.0.0.0/21",
		ServiceCIDR:       "10.1.0.0/16",
		Location:          "FIN-03",
		NetworkID:         "net_123",
		SubnetID:          "sub_123",
	}
	pool := CreateManagedKubernetesNodePoolRequest{
		InstanceType: "CPU.4V.16",
		SSHKeyIDs:    []string{"key_1"},
	}

	for _, tc := range []struct {
		length  int
		wantErr bool
	}{
		{length: 30, wantErr: false},
		{length: 31, wantErr: true},
	} {
		name := strings.Repeat("a", tc.length)

		cluster.Name = name
		if err := cluster.Validate(); (err != nil) != tc.wantErr {
			t.Errorf("cluster name of %d characters: Validate() error = %v, wantErr %v", tc.length, err, tc.wantErr)
		}

		pool.Name = name
		if err := pool.Validate(); (err != nil) != tc.wantErr {
			t.Errorf("pool name of %d characters: Validate() error = %v, wantErr %v", tc.length, err, tc.wantErr)
		}
	}
}

func TestManagedKubernetesService_PathValidation(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"empty location", func() error { _, err := client.ManagedKubernetes.Get(ctx, "", "my-cluster"); return err }},
		{"dot location", func() error { _, err := client.ManagedKubernetes.Get(ctx, ".", "my-cluster"); return err }},
		{"dot-dot location", func() error { _, err := client.ManagedKubernetes.Get(ctx, "..", "my-cluster"); return err }},
		{"empty cluster", func() error { _, err := client.ManagedKubernetes.Get(ctx, "FIN-03", ""); return err }},
		{"cluster with slash", func() error { _, err := client.ManagedKubernetes.Get(ctx, "FIN-03", "a/b"); return err }},
		{"cluster dot-dot", func() error { return client.ManagedKubernetes.Delete(ctx, "FIN-03", "..") }},
		{"cluster with query", func() error { _, err := client.ManagedKubernetes.Get(ctx, "FIN-03", "a?x=1"); return err }},
		{"cluster uppercase", func() error { _, err := client.ManagedKubernetes.Get(ctx, "FIN-03", "MyCluster"); return err }},
		{"cluster too long", func() error {
			_, err := client.ManagedKubernetes.Get(ctx, "FIN-03", strings.Repeat("a", 64))
			return err
		}},
		{"empty pool", func() error { _, err := client.ManagedKubernetes.GetNodePool(ctx, "FIN-03", "c", ""); return err }},
		{"pool traversal", func() error {
			_, err := client.ManagedKubernetes.GetNodePool(ctx, "FIN-03", "c", "../../other")
			return err
		}},
		{"pool too long", func() error {
			return client.ManagedKubernetes.DeleteNodePool(ctx, "FIN-03", "c", strings.Repeat("a", 64))
		}},
		{"empty kubeconfig id", func() error { _, err := client.ManagedKubernetes.GetKubeconfig(ctx, "FIN-03", "c", ""); return err }},
		{"dot-dot kubeconfig id", func() error { return client.ManagedKubernetes.DeleteKubeconfig(ctx, "FIN-03", "c", "..") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			var apiErr *APIError
			if errors.As(err, &apiErr) {
				t.Fatalf("expected the request to be rejected before sending, got API error %v", apiErr)
			}
		})
	}

	// The API checks the 30-character limit on create only, so paths of
	// existing resources with longer names must still be sent.
	t.Run("names over the create limit are sent", func(t *testing.T) {
		longCluster := strings.Repeat("c", managedKubernetesNameMaxLength+1)
		longPool := strings.Repeat("p", managedKubernetesNameMaxLength+1)

		var clusterHit, poolHit bool
		mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/"+longCluster, func(w http.ResponseWriter, r *http.Request) {
			clusterHit = true
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(testClusterJSON)); err != nil {
				t.Errorf("write mock response: %v", err)
			}
		})
		mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN-03/"+longCluster+"/pools/"+longPool, func(w http.ResponseWriter, r *http.Request) {
			poolHit = true
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(testPoolJSON)); err != nil {
				t.Errorf("write mock response: %v", err)
			}
		})

		if _, err := client.ManagedKubernetes.Get(ctx, "FIN-03", longCluster); err != nil {
			t.Fatalf("Get with a %d-character cluster name: %v", len(longCluster), err)
		}
		if _, err := client.ManagedKubernetes.GetNodePool(ctx, "FIN-03", longCluster, longPool); err != nil {
			t.Fatalf("GetNodePool with a %d-character pool name: %v", len(longPool), err)
		}
		if !clusterHit || !poolHit {
			t.Errorf("expected both requests to reach the server (cluster=%v, pool=%v)", clusterHit, poolHit)
		}
	})

	t.Run("segments are escaped", func(t *testing.T) {
		var escapedPath string
		mockServer.SetHandler(http.MethodGet, "/mk8sclusters/FIN 03/c/kubeconfigs/a/b", func(w http.ResponseWriter, r *http.Request) {
			escapedPath = r.URL.EscapedPath()
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(testKubeconfigReadyJSON)); err != nil {
				t.Errorf("write mock response: %v", err)
			}
		})

		if _, err := client.ManagedKubernetes.GetKubeconfig(ctx, "FIN 03", "c", "a/b"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "/mk8sclusters/FIN%2003/c/kubeconfigs/a%2Fb"; escapedPath != want {
			t.Errorf("escaped path = %q, want %q", escapedPath, want)
		}
	})
}

func TestManagedKubernetesKubeconfig_FormatRedactsKubeconfig(t *testing.T) {
	const secret = "PRIVATE-KEY-MATERIAL"
	kc := ManagedKubernetesKubeconfig{ID: "kc-1", CommonName: "alice", Status: ManagedKubernetesKubeconfigStatusReady, Kubeconfig: secret}

	outputs := map[string]string{
		"%v value":   fmt.Sprintf("%v", kc),
		"%+v value":  fmt.Sprintf("%+v", kc),
		"%#v value":  fmt.Sprintf("%#v", kc),
		"%s value":   fmt.Sprintf("%s", kc),
		"%v pointer": fmt.Sprintf("%v", &kc),
		"%#v ptr":    fmt.Sprintf("%#v", &kc),
		"%v slice":   fmt.Sprintf("%v", []ManagedKubernetesKubeconfig{kc}),
		"%v wrapped": fmt.Errorf("failed: %v", kc).Error(),
		"%d value":   fmt.Sprintf("%d", kc),
		"%q value":   fmt.Sprintf("%q", kc),
		"%x value":   fmt.Sprintf("%x", kc),
		"%10v value": fmt.Sprintf("%10v", kc),
		"%d pointer": fmt.Sprintf("%d", &kc),
		"%d slice":   fmt.Sprintf("%d", []ManagedKubernetesKubeconfig{kc}),
	}
	hexSecret := fmt.Sprintf("%x", secret)
	for name, out := range outputs {
		if strings.Contains(out, secret) || strings.Contains(out, hexSecret) {
			t.Errorf("%s leaks the kubeconfig: %s", name, out)
		}
		readable := !strings.HasPrefix(name, "%d") && !strings.HasPrefix(name, "%x")
		if readable && (!strings.Contains(out, "kc-1") || !strings.Contains(out, "[REDACTED]")) {
			t.Errorf("%s should show the ID and a redaction marker, got: %s", name, out)
		}
	}

	if got := fmt.Sprintf("%#v", kc); !strings.HasPrefix(got, "verda.ManagedKubernetesKubeconfig{") {
		t.Errorf("%%#v should name the public type, got %s", got)
	}

	if out := fmt.Sprintf("%v", ManagedKubernetesKubeconfig{ID: "kc-2"}); strings.Contains(out, "[REDACTED]") {
		t.Errorf("an empty kubeconfig should not show a redaction marker, got %s", out)
	}

	body, err := json.Marshal(kc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), secret) {
		t.Errorf("JSON encoding must keep the kubeconfig, got %s", body)
	}
}
