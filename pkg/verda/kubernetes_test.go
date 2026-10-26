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
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

const testKubernetesProjectID = "11111111-1111-4111-8111-111111111111"

// testKubernetesClusterPath is the route for the "demo" cluster used across tests
const testKubernetesClusterPath = kubernetesClustersBasePath + "/demo"

// Captured from a local API create response
const provisioningClusterJSON = `{
  "name": "demo",
  "kubernetes_version": "1.35.8",
  "location": "FIN-01",
  "network": "pn_local",
  "subnet": "sn_local_fin01",
  "control_plane_profile": "standard",
  "phase": "provisioning",
  "created_at": "2026-10-22T13:40:37+03:00"
}`

const runningClusterJSON = `{
  "name": "demo",
  "kubernetes_version": "1.35.8",
  "location": "FIN-01",
  "network": "pn_local",
  "subnet": "sn_local_fin01",
  "control_plane_profile": "standard",
  "phase": "running",
  "external_endpoint": "https://203.0.113.10:6443",
  "conditions": [
    {
      "type": "Ready",
      "status": "True",
      "observedGeneration": 2,
      "lastTransitionTime": "2026-10-22T13:50:00Z",
      "reason": "AllComponentsReady",
      "message": "ok"
    }
  ],
  "created_at": "2026-10-22T13:40:37+03:00"
}`

func validCreateKubernetesClusterRequest() CreateKubernetesClusterRequest {
	return CreateKubernetesClusterRequest{
		Name:              "demo",
		KubernetesVersion: "1.35.8",
		PodCIDR:           "10.244.0.0/16",
		ServiceCIDR:       "10.96.0.0/12",
		Location:          "FIN-01",
		NetworkID:         "pn_local",
		SubnetID:          "sn_local_fin01",
	}
}

// capturedRequest records what the mock server received
type capturedRequest struct {
	method    string
	path      string
	projectID string
	body      map[string]any
}

func captureHandler(t *testing.T, got *capturedRequest, status int, response string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.EscapedPath()
		got.projectID = r.URL.Query().Get("projectId")
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &got.body); err != nil {
				t.Errorf("request body is not JSON: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}
}

func TestKubernetesService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodPost, kubernetesClustersBasePath, captureHandler(t, &got, http.StatusCreated, provisioningClusterJSON))

	cluster, err := client.Kubernetes.Create(context.Background(), testKubernetesProjectID, validCreateKubernetesClusterRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.method != http.MethodPost || got.path != kubernetesClustersBasePath {
		t.Errorf("unexpected request %s %s", got.method, got.path)
	}
	if got.projectID != testKubernetesProjectID {
		t.Errorf("expected projectId %q, got %q", testKubernetesProjectID, got.projectID)
	}
	want := map[string]string{
		"name":               "demo",
		"kubernetes_version": "1.35.8",
		"pod_cidr":           "10.244.0.0/16",
		"service_cidr":       "10.96.0.0/12",
		"location":           "FIN-01",
		"network":            "pn_local",
		"subnet":             "sn_local_fin01",
	}
	if len(got.body) != len(want) {
		t.Errorf("expected body keys %v, got %v", want, got.body)
	}
	for k, v := range want {
		if got.body[k] != v {
			t.Errorf("expected body %s=%q, got %v", k, v, got.body[k])
		}
	}
	if _, ok := got.body["control_plane_profile"]; ok {
		t.Error("expected empty control_plane_profile to be omitted")
	}

	if cluster.Phase != KubernetesClusterPhaseProvisioning {
		t.Errorf("expected phase provisioning, got %q", cluster.Phase)
	}
	if cluster.NetworkID != "pn_local" || cluster.SubnetID != "sn_local_fin01" {
		t.Errorf("expected network/subnet to decode, got %q/%q", cluster.NetworkID, cluster.SubnetID)
	}
	if cluster.ExternalEndpoint != "" || cluster.Conditions != nil {
		t.Errorf("expected no endpoint or conditions while provisioning, got %+v", cluster)
	}
	if cluster.CreatedAt.IsZero() {
		t.Error("expected created_at to decode")
	}
}

func TestKubernetesService_CreateWithProfile(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodPost, kubernetesClustersBasePath, captureHandler(t, &got, http.StatusCreated, provisioningClusterJSON))

	req := validCreateKubernetesClusterRequest()
	req.ControlPlaneProfile = KubernetesControlPlaneProfileLarge
	if _, err := client.Kubernetes.Create(context.Background(), testKubernetesProjectID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.body["control_plane_profile"] != "large" {
		t.Errorf("expected control_plane_profile large, got %v", got.body["control_plane_profile"])
	}
}

func TestKubernetesService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodGet, testKubernetesClusterPath, captureHandler(t, &got, http.StatusOK, runningClusterJSON))

	cluster, err := client.Kubernetes.Get(context.Background(), testKubernetesProjectID, "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.method != http.MethodGet || got.projectID != testKubernetesProjectID {
		t.Errorf("unexpected request %s projectId=%q", got.method, got.projectID)
	}
	if cluster.Phase != KubernetesClusterPhaseRunning || cluster.ExternalEndpoint != "https://203.0.113.10:6443" {
		t.Errorf("unexpected cluster %+v", cluster)
	}
	if len(cluster.Conditions) != 1 {
		t.Fatalf("expected one condition, got %d", len(cluster.Conditions))
	}
	c := cluster.Conditions[0]
	if c.Type != "Ready" || c.Status != "True" || c.ObservedGeneration != 2 || c.Reason != "AllComponentsReady" || c.LastTransitionTime.IsZero() {
		t.Errorf("unexpected condition %+v", c)
	}
}

func TestKubernetesService_GetUnknownPhase(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodGet, testKubernetesClusterPath,
		captureHandler(t, &got, http.StatusOK, `{"name":"demo","phase":"hibernating","created_at":"2026-10-22T13:40:37Z"}`))

	cluster, err := client.Kubernetes.Get(context.Background(), testKubernetesProjectID, "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cluster.Phase != "hibernating" {
		t.Errorf("expected unknown phase to be preserved, got %q", cluster.Phase)
	}
}

func TestKubernetesClusterPath(t *testing.T) {
	const query = "?projectId=" + testKubernetesProjectID

	cases := []struct {
		name  string
		names []string
		want  string
	}{
		{"collection", nil, kubernetesClustersBasePath + query},
		{"cluster", []string{"demo"}, kubernetesClustersBasePath + "/demo" + query},
		{"slash stays in one segment", []string{"a/b"}, kubernetesClustersBasePath + "/a%2Fb" + query},
		{"query cannot be injected", []string{"demo?projectId=other"}, kubernetesClustersBasePath + "/demo%3FprojectId=other" + query},
		{"fragment", []string{"a#b"}, kubernetesClustersBasePath + "/a%23b" + query},
		{"space", []string{"my demo"}, kubernetesClustersBasePath + "/my%20demo" + query},
		{"percent", []string{"50%"}, kubernetesClustersBasePath + "/50%25" + query},
		{"non-ascii", []string{"démo"}, kubernetesClustersBasePath + "/d%C3%A9mo" + query},
		{"nested segments", []string{"demo", "pools", "workers"}, kubernetesClustersBasePath + "/demo/pools/workers" + query},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := kubernetesClusterPath(testKubernetesProjectID, tc.names...); got != tc.want {
				t.Errorf("kubernetesClusterPath(%q) = %q, want %q", tc.names, got, tc.want)
			}
		})
	}
}

func TestKubernetesService_GetEscapesName(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodGet, kubernetesClustersBasePath+"/a/b", captureHandler(t, &got, http.StatusOK, provisioningClusterJSON))
	mockServer.SetHandler(http.MethodGet, kubernetesClustersBasePath+"/a%2Fb", captureHandler(t, &got, http.StatusOK, provisioningClusterJSON))

	if _, err := client.Kubernetes.Get(context.Background(), testKubernetesProjectID, "a/b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.path != kubernetesClustersBasePath+"/a%2Fb" {
		t.Errorf("expected escaped name in path, got %q", got.path)
	}
}

func TestKubernetesService_Update(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodPatch, testKubernetesClusterPath, captureHandler(t, &got, http.StatusOK, provisioningClusterJSON))

	_, err := client.Kubernetes.Update(context.Background(), testKubernetesProjectID, "demo",
		UpdateKubernetesClusterRequest{KubernetesVersion: "1.36.4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.method != http.MethodPatch || got.projectID != testKubernetesProjectID {
		t.Errorf("unexpected request %s projectId=%q", got.method, got.projectID)
	}
	if len(got.body) != 1 || got.body["kubernetes_version"] != "1.36.4" {
		t.Errorf("expected only kubernetes_version in body, got %v", got.body)
	}
}

func TestKubernetesService_Delete(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodDelete, testKubernetesClusterPath, captureHandler(t, &got, http.StatusAccepted, ""))

	if err := client.Kubernetes.Delete(context.Background(), testKubernetesProjectID, "demo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.method != http.MethodDelete || got.projectID != testKubernetesProjectID {
		t.Errorf("unexpected request %s projectId=%q", got.method, got.projectID)
	}
}

func TestKubernetesService_ValidationBeforeHTTP(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	var calls atomic.Int32
	count := func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}
	mockServer.SetHandler(http.MethodPost, kubernetesClustersBasePath, count)
	mockServer.SetHandler(http.MethodGet, testKubernetesClusterPath, count)
	mockServer.SetHandler(http.MethodPatch, testKubernetesClusterPath, count)
	mockServer.SetHandler(http.MethodDelete, testKubernetesClusterPath, count)

	mutate := func(f func(*CreateKubernetesClusterRequest)) CreateKubernetesClusterRequest {
		req := validCreateKubernetesClusterRequest()
		f(&req)
		return req
	}
	createCases := map[string]CreateKubernetesClusterRequest{
		"missing name":         mutate(func(r *CreateKubernetesClusterRequest) { r.Name = "" }),
		"missing version":      mutate(func(r *CreateKubernetesClusterRequest) { r.KubernetesVersion = "" }),
		"missing pod cidr":     mutate(func(r *CreateKubernetesClusterRequest) { r.PodCIDR = "" }),
		"invalid pod cidr":     mutate(func(r *CreateKubernetesClusterRequest) { r.PodCIDR = "10.244.0.0" }),
		"ipv6 service cidr":    mutate(func(r *CreateKubernetesClusterRequest) { r.ServiceCIDR = "fd00::/108" }),
		"missing service cidr": mutate(func(r *CreateKubernetesClusterRequest) { r.ServiceCIDR = "" }),
		"missing location":     mutate(func(r *CreateKubernetesClusterRequest) { r.Location = "" }),
		"missing network":      mutate(func(r *CreateKubernetesClusterRequest) { r.NetworkID = "" }),
		"missing subnet":       mutate(func(r *CreateKubernetesClusterRequest) { r.SubnetID = "" }),
		"invalid profile":      mutate(func(r *CreateKubernetesClusterRequest) { r.ControlPlaneProfile = "huge" }),
	}
	for name, req := range createCases {
		t.Run("create "+name, func(t *testing.T) {
			if _, err := client.Kubernetes.Create(ctx, testKubernetesProjectID, req); err == nil {
				t.Error("expected validation error")
			}
		})
	}

	for name, projectID := range map[string]string{"empty project": "", "non-uuid project": "my-project"} {
		t.Run(name, func(t *testing.T) {
			if _, err := client.Kubernetes.Create(ctx, projectID, validCreateKubernetesClusterRequest()); err == nil {
				t.Error("expected create error")
			}
			if _, err := client.Kubernetes.Get(ctx, projectID, "demo"); err == nil {
				t.Error("expected get error")
			}
			if _, err := client.Kubernetes.Update(ctx, projectID, "demo", UpdateKubernetesClusterRequest{KubernetesVersion: "1.36.4"}); err == nil {
				t.Error("expected update error")
			}
			if err := client.Kubernetes.Delete(ctx, projectID, "demo"); err == nil {
				t.Error("expected delete error")
			}
		})
	}

	t.Run("empty name", func(t *testing.T) {
		if _, err := client.Kubernetes.Get(ctx, testKubernetesProjectID, ""); err == nil {
			t.Error("expected get error")
		}
		if _, err := client.Kubernetes.Update(ctx, testKubernetesProjectID, "", UpdateKubernetesClusterRequest{KubernetesVersion: "1.36.4"}); err == nil {
			t.Error("expected update error")
		}
		if err := client.Kubernetes.Delete(ctx, testKubernetesProjectID, ""); err == nil {
			t.Error("expected delete error")
		}
	})

	t.Run("update missing version", func(t *testing.T) {
		if _, err := client.Kubernetes.Update(ctx, testKubernetesProjectID, "demo", UpdateKubernetesClusterRequest{}); err == nil {
			t.Error("expected validation error")
		}
	})

	if n := calls.Load(); n != 0 {
		t.Errorf("expected no HTTP calls for invalid input, got %d", n)
	}
}

func TestKubernetesService_SelectorValidationErrorFields(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	cases := []struct {
		name      string
		projectID string
		cluster   string
		wantField string
	}{
		{"non-uuid project", "my-project", "demo", "projectID"},
		{"empty name", testKubernetesProjectID, "", "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Kubernetes.Get(ctx, tc.projectID, tc.cluster)
			var vErr *ValidationError
			if !errors.As(err, &vErr) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if vErr.Field != tc.wantField {
				t.Errorf("expected field %q, got %q", tc.wantField, vErr.Field)
			}
		})
	}
}

func TestKubernetesService_APIErrors(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	cases := []struct {
		name   string
		status int
		body   string
	}{
		{
			"structured error envelope",
			http.StatusNotFound,
			`{"code":"not_found","message":"resource not found","details":{"reason":"resource_not_found"}}`,
		},
		{"not found", http.StatusNotFound, `{"message":"cluster not found"}`},
		{"admission rejected", http.StatusBadRequest, `{"message":"unknown kubernetes version"}`},
		{"feature disabled", http.StatusForbidden, `{"message":"feature not enabled for this project"}`},
		{"not a project member", http.StatusForbidden, `{"error":"forbidden"}`},
		{"proxy html", http.StatusBadGateway, `<html><body>Bad Gateway</body></html>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got capturedRequest
			mockServer.SetHandler(http.MethodGet, testKubernetesClusterPath, func(w http.ResponseWriter, r *http.Request) {
				got.projectID = r.URL.Query().Get("projectId")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})

			cluster, err := client.Kubernetes.Get(ctx, testKubernetesProjectID, "demo")
			if err == nil {
				t.Fatalf("expected error, got cluster %+v", cluster)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tc.status {
				t.Errorf("expected status %d, got %d", tc.status, apiErr.StatusCode)
			}
			if apiErr.Message != tc.body || apiErr.Code != "" || apiErr.Details != "" {
				t.Errorf("expected raw response body in Message and empty Code/Details, got %+v", apiErr)
			}
		})
	}

	createCall := func() (*KubernetesCluster, error) {
		return client.Kubernetes.Create(ctx, testKubernetesProjectID, validCreateKubernetesClusterRequest())
	}
	updateCall := func() (*KubernetesCluster, error) {
		return client.Kubernetes.Update(ctx, testKubernetesProjectID, "demo", UpdateKubernetesClusterRequest{KubernetesVersion: "1.36.4"})
	}
	mutationCases := []struct {
		name   string
		method string
		path   string
		call   func() (*KubernetesCluster, error)
		status int
		body   string
	}{
		{
			"create already exists",
			http.MethodPost,
			kubernetesClustersBasePath,
			createCall,
			http.StatusConflict,
			`{"code":"conflict","message":"resource already exists","details":{"reason":"resource_already_exists"}}`,
		},
		{"create foreign network", http.MethodPost, kubernetesClustersBasePath, createCall,
			http.StatusBadRequest, `{"message":"network \"pn_other_project\" not found"}`},
		{"update immutable network", http.MethodPatch, testKubernetesClusterPath, updateCall,
			http.StatusBadRequest, `{"message":"network and subnet are immutable"}`},
		{
			"update not found",
			http.MethodPatch,
			testKubernetesClusterPath,
			updateCall,
			http.StatusNotFound,
			`{"code":"not_found","message":"resource not found","details":{"reason":"resource_not_found"}}`,
		},
	}
	for _, tc := range mutationCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer.SetHandler(tc.method, tc.path, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})

			cluster, err := tc.call()
			if cluster != nil {
				t.Errorf("expected nil cluster on error, got %+v", cluster)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tc.status {
				t.Errorf("expected status %d, got %d", tc.status, apiErr.StatusCode)
			}
			if apiErr.Message != tc.body || apiErr.Code != "" || apiErr.Details != "" {
				t.Errorf("expected raw response body in Message and empty Code/Details, got %+v", apiErr)
			}
		})
	}

	t.Run("delete not found", func(t *testing.T) {
		body := `{"code":"not_found","message":"resource not found","details":{"reason":"resource_not_found"}}`
		mockServer.SetHandler(http.MethodDelete, testKubernetesClusterPath, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(body))
		})
		err := client.Kubernetes.Delete(ctx, testKubernetesProjectID, "demo")
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 *APIError, got %v", err)
		}
		if apiErr.Message != body || apiErr.Code != "" || apiErr.Details != "" {
			t.Errorf("expected raw response body in Message and empty Code/Details, got %+v", apiErr)
		}
	})
}

func TestKubernetesService_DoesNotAffectComputeClusters(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	if client.Kubernetes == nil {
		t.Fatal("expected Kubernetes service to be wired")
	}
	if client.Clusters == nil {
		t.Fatal("expected compute Clusters service to remain wired")
	}
	if _, err := client.Clusters.Get(context.Background()); err != nil {
		t.Errorf("compute clusters call failed: %v", err)
	}
}

func TestCreateKubernetesClusterRequest_NameValidation(t *testing.T) {
	withName := func(name string) CreateKubernetesClusterRequest {
		req := validCreateKubernetesClusterRequest()
		req.Name = name
		return req
	}

	// The 30-character backend limit is deliberately not enforced by the SDK
	valid := map[string]string{
		"single letter":     "a",
		"single digit":      "1",
		"digits only":       "123",
		"leading digit":     "1demo",
		"inner hyphens":     "my-demo--cluster",
		"31 characters":     strings.Repeat("a", 31),
		"63 characters max": strings.Repeat("a", 63),
	}
	for name, clusterName := range valid {
		t.Run("valid "+name, func(t *testing.T) {
			if err := withName(clusterName).Validate(); err != nil {
				t.Errorf("expected %q to be valid, got %v", clusterName, err)
			}
		})
	}

	invalid := map[string]string{
		"uppercase":         "Demo",
		"underscore":        "my_cluster",
		"dot":               "demo.prod",
		"leading hyphen":    "-demo",
		"trailing hyphen":   "demo-",
		"whitespace":        "my demo",
		"surrounding space": " demo ",
		"non-ascii":         "démo",
		"64 characters":     strings.Repeat("a", 64),
	}
	for name, clusterName := range invalid {
		t.Run("invalid "+name, func(t *testing.T) {
			err := withName(clusterName).Validate()
			var errs validation.Errors
			if !errors.As(err, &errs) || errs["name"] == nil {
				t.Errorf("expected a name validation error for %q, got %v", clusterName, err)
			}
		})
	}
}
