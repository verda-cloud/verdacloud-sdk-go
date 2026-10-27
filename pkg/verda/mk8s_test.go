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
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

const testMK8SProjectID = "11111111-1111-4111-8111-111111111111"

// testMK8SClusterPath is the route for the "demo" cluster used across tests
const testMK8SClusterPath = mk8sClustersBasePath + "/demo"

// Example API create response
const provisioningClusterJSON = `{
  "name": "demo",
  "kubernetes_version": "1.35.8",
  "location": "FIN-01",
  "network": "pn_local",
  "subnet": "sn_local_fin01",
  "phase": "provisioning",
  "created_at": "2026-10-22T13:40:37+03:00"
}`

const runningClusterJSON = `{
  "name": "demo",
  "kubernetes_version": "1.35.8",
  "location": "FIN-01",
  "network": "pn_local",
  "subnet": "sn_local_fin01",
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

func validCreateMK8SClusterRequest() CreateMK8SClusterRequest {
	return CreateMK8SClusterRequest{
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

func TestMK8SService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodPost, mk8sClustersBasePath, captureHandler(t, &got, http.StatusCreated, provisioningClusterJSON))

	cluster, err := client.MK8S.Create(context.Background(), testMK8SProjectID, validCreateMK8SClusterRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.method != http.MethodPost || got.path != mk8sClustersBasePath {
		t.Errorf("unexpected request %s %s", got.method, got.path)
	}
	if got.projectID != testMK8SProjectID {
		t.Errorf("expected projectId %q, got %q", testMK8SProjectID, got.projectID)
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
		t.Error("expected control_plane_profile to be omitted")
	}

	if cluster.Phase != MK8SClusterPhaseProvisioning {
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

func TestMK8SService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodGet, testMK8SClusterPath, captureHandler(t, &got, http.StatusOK, runningClusterJSON))

	cluster, err := client.MK8S.Get(context.Background(), testMK8SProjectID, "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.method != http.MethodGet || got.projectID != testMK8SProjectID {
		t.Errorf("unexpected request %s projectId=%q", got.method, got.projectID)
	}
	if cluster.Phase != MK8SClusterPhaseRunning || cluster.ExternalEndpoint != "https://203.0.113.10:6443" {
		t.Errorf("unexpected cluster %+v", cluster)
	}
	if len(cluster.Conditions) != 1 {
		t.Fatalf("expected one condition, got %d", len(cluster.Conditions))
	}
	c := cluster.Conditions[0]
	if c.Type != "Ready" || c.Status != "True" || c.ObservedGeneration != 2 || c.Reason != "AllComponentsReady" || c.Message != "ok" ||
		!c.LastTransitionTime.Equal(time.Date(2026, time.October, 22, 13, 50, 0, 0, time.UTC)) {
		t.Errorf("unexpected condition %+v", c)
	}
}

func TestMK8SService_GetPhases(t *testing.T) {
	for _, phase := range []MK8SClusterPhase{MK8SClusterPhaseDegraded, "hibernating"} {
		t.Run(string(phase), func(t *testing.T) {
			mockServer := testutil.NewMockServer()
			defer mockServer.Close()
			client := NewTestClient(t, mockServer)

			var got capturedRequest
			mockServer.SetHandler(http.MethodGet, testMK8SClusterPath,
				captureHandler(t, &got, http.StatusOK, `{"name":"demo","phase":"`+string(phase)+`","created_at":"2026-10-22T13:40:37Z"}`))

			cluster, err := client.MK8S.Get(context.Background(), testMK8SProjectID, "demo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cluster.Phase != phase {
				t.Errorf("expected phase %q to be preserved, got %q", phase, cluster.Phase)
			}
		})
	}
}

func TestMK8SClusterPath(t *testing.T) {
	const query = "?projectId=" + testMK8SProjectID

	cases := []struct {
		name  string
		names []string
		want  string
	}{
		{"collection", nil, mk8sClustersBasePath + query},
		{"cluster", []string{"demo"}, mk8sClustersBasePath + "/demo" + query},
		{"slash stays in one segment", []string{"a/b"}, mk8sClustersBasePath + "/a%2Fb" + query},
		{"query cannot be injected", []string{"demo?projectId=other"}, mk8sClustersBasePath + "/demo%3FprojectId=other" + query},
		{"fragment", []string{"a#b"}, mk8sClustersBasePath + "/a%23b" + query},
		{"space", []string{"my demo"}, mk8sClustersBasePath + "/my%20demo" + query},
		{"percent", []string{"50%"}, mk8sClustersBasePath + "/50%25" + query},
		{"non-ascii", []string{"démo"}, mk8sClustersBasePath + "/d%C3%A9mo" + query},
		{"nested segments", []string{"demo", "pools", "workers"}, mk8sClustersBasePath + "/demo/pools/workers" + query},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mk8sClusterPath(testMK8SProjectID, tc.names...); got != tc.want {
				t.Errorf("mk8sClusterPath(%q) = %q, want %q", tc.names, got, tc.want)
			}
		})
	}
}

func TestMK8SService_GetEscapesName(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodGet, mk8sClustersBasePath+"/a/b", captureHandler(t, &got, http.StatusOK, provisioningClusterJSON))
	mockServer.SetHandler(http.MethodGet, mk8sClustersBasePath+"/a%2Fb", captureHandler(t, &got, http.StatusOK, provisioningClusterJSON))

	if _, err := client.MK8S.Get(context.Background(), testMK8SProjectID, "a/b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.path != mk8sClustersBasePath+"/a%2Fb" {
		t.Errorf("expected escaped name in path, got %q", got.path)
	}
}

func TestMK8SService_Update(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodPatch, testMK8SClusterPath, captureHandler(t, &got, http.StatusOK, provisioningClusterJSON))

	_, err := client.MK8S.Update(context.Background(), testMK8SProjectID, "demo",
		UpdateMK8SClusterRequest{KubernetesVersion: "1.36.4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.method != http.MethodPatch || got.projectID != testMK8SProjectID {
		t.Errorf("unexpected request %s projectId=%q", got.method, got.projectID)
	}
	if len(got.body) != 1 || got.body["kubernetes_version"] != "1.36.4" {
		t.Errorf("expected only kubernetes_version in body, got %v", got.body)
	}
}

func TestMK8SService_Delete(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var got capturedRequest
	mockServer.SetHandler(http.MethodDelete, testMK8SClusterPath, captureHandler(t, &got, http.StatusAccepted, ""))

	if err := client.MK8S.Delete(context.Background(), testMK8SProjectID, "demo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.method != http.MethodDelete || got.projectID != testMK8SProjectID {
		t.Errorf("unexpected request %s projectId=%q", got.method, got.projectID)
	}
}

func TestMK8SService_ValidationBeforeHTTP(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	var calls atomic.Int32
	count := func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}
	mockServer.SetHandler(http.MethodPost, mk8sClustersBasePath, count)
	mockServer.SetHandler(http.MethodGet, testMK8SClusterPath, count)
	mockServer.SetHandler(http.MethodPatch, testMK8SClusterPath, count)
	mockServer.SetHandler(http.MethodDelete, testMK8SClusterPath, count)

	mutate := func(f func(*CreateMK8SClusterRequest)) CreateMK8SClusterRequest {
		req := validCreateMK8SClusterRequest()
		f(&req)
		return req
	}
	createCases := map[string]CreateMK8SClusterRequest{
		"missing name":         mutate(func(r *CreateMK8SClusterRequest) { r.Name = "" }),
		"name exceeds maximum": mutate(func(r *CreateMK8SClusterRequest) { r.Name = strings.Repeat("a", MK8SClusterNameMaxLength+1) }),
		"invalid name":         mutate(func(r *CreateMK8SClusterRequest) { r.Name = "Demo" }),
		"missing version":      mutate(func(r *CreateMK8SClusterRequest) { r.KubernetesVersion = "" }),
		"missing pod cidr":     mutate(func(r *CreateMK8SClusterRequest) { r.PodCIDR = "" }),
		"invalid pod cidr":     mutate(func(r *CreateMK8SClusterRequest) { r.PodCIDR = "10.244.0.0" }),
		"ipv6 service cidr":    mutate(func(r *CreateMK8SClusterRequest) { r.ServiceCIDR = "fd00::/108" }),
		"missing service cidr": mutate(func(r *CreateMK8SClusterRequest) { r.ServiceCIDR = "" }),
		"missing location":     mutate(func(r *CreateMK8SClusterRequest) { r.Location = "" }),
		"missing network":      mutate(func(r *CreateMK8SClusterRequest) { r.NetworkID = "" }),
		"missing subnet":       mutate(func(r *CreateMK8SClusterRequest) { r.SubnetID = "" }),
	}
	for name, req := range createCases {
		t.Run("create "+name, func(t *testing.T) {
			_, err := client.MK8S.Create(ctx, testMK8SProjectID, req)
			var errs validation.Errors
			if !errors.As(err, &errs) {
				t.Errorf("expected ozzo-validation errors, got %T: %v", err, err)
			}
		})
	}

	for name, projectID := range map[string]string{"empty project": "", "non-uuid project": "my-project"} {
		t.Run(name, func(t *testing.T) {
			if _, err := client.MK8S.Create(ctx, projectID, validCreateMK8SClusterRequest()); err == nil {
				t.Error("expected create error")
			}
			if _, err := client.MK8S.Get(ctx, projectID, "demo"); err == nil {
				t.Error("expected get error")
			}
			if _, err := client.MK8S.Update(ctx, projectID, "demo", UpdateMK8SClusterRequest{KubernetesVersion: "1.36.4"}); err == nil {
				t.Error("expected update error")
			}
			if err := client.MK8S.Delete(ctx, projectID, "demo"); err == nil {
				t.Error("expected delete error")
			}
		})
	}

	t.Run("empty name", func(t *testing.T) {
		if _, err := client.MK8S.Get(ctx, testMK8SProjectID, ""); err == nil {
			t.Error("expected get error")
		}
		if _, err := client.MK8S.Update(ctx, testMK8SProjectID, "", UpdateMK8SClusterRequest{KubernetesVersion: "1.36.4"}); err == nil {
			t.Error("expected update error")
		}
		if err := client.MK8S.Delete(ctx, testMK8SProjectID, ""); err == nil {
			t.Error("expected delete error")
		}
	})

	t.Run("update missing version", func(t *testing.T) {
		if _, err := client.MK8S.Update(ctx, testMK8SProjectID, "demo", UpdateMK8SClusterRequest{}); err == nil {
			t.Error("expected validation error")
		}
	})

	if n := calls.Load(); n != 0 {
		t.Errorf("expected no HTTP calls for invalid input, got %d", n)
	}
}

func TestMK8SService_SelectorValidationErrorFields(t *testing.T) {
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
		{"empty name", testMK8SProjectID, "", "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.MK8S.Get(ctx, tc.projectID, tc.cluster)
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

func TestMK8SService_APIErrors(t *testing.T) {
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
			mockServer.SetHandler(http.MethodGet, testMK8SClusterPath, func(w http.ResponseWriter, r *http.Request) {
				got.projectID = r.URL.Query().Get("projectId")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})

			cluster, err := client.MK8S.Get(ctx, testMK8SProjectID, "demo")
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

	createCall := func() (*MK8SCluster, error) {
		return client.MK8S.Create(ctx, testMK8SProjectID, validCreateMK8SClusterRequest())
	}
	updateCall := func() (*MK8SCluster, error) {
		return client.MK8S.Update(ctx, testMK8SProjectID, "demo", UpdateMK8SClusterRequest{KubernetesVersion: "1.36.4"})
	}
	mutationCases := []struct {
		name   string
		method string
		path   string
		call   func() (*MK8SCluster, error)
		status int
		body   string
	}{
		{
			"create already exists",
			http.MethodPost,
			mk8sClustersBasePath,
			createCall,
			http.StatusConflict,
			`{"code":"conflict","message":"resource already exists","details":{"reason":"resource_already_exists"}}`,
		},
		{"create foreign network", http.MethodPost, mk8sClustersBasePath, createCall,
			http.StatusBadRequest, `{"message":"network \"pn_other_project\" not found"}`},
		{"update immutable network", http.MethodPatch, testMK8SClusterPath, updateCall,
			http.StatusBadRequest, `{"message":"network and subnet are immutable"}`},
		{
			"update not found",
			http.MethodPatch,
			testMK8SClusterPath,
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
		mockServer.SetHandler(http.MethodDelete, testMK8SClusterPath, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(body))
		})
		err := client.MK8S.Delete(ctx, testMK8SProjectID, "demo")
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 *APIError, got %v", err)
		}
		if apiErr.Message != body || apiErr.Code != "" || apiErr.Details != "" {
			t.Errorf("expected raw response body in Message and empty Code/Details, got %+v", apiErr)
		}
	})
}

func TestMK8SService_DoesNotAffectComputeClusters(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	if client.MK8S == nil {
		t.Fatal("expected MK8S service to be wired")
	}
	if client.Clusters == nil {
		t.Fatal("expected compute Clusters service to remain wired")
	}
	if _, err := client.Clusters.Get(context.Background()); err != nil {
		t.Errorf("compute clusters call failed: %v", err)
	}
}

func TestCreateMK8SClusterRequest_NameValidation(t *testing.T) {
	withName := func(name string) CreateMK8SClusterRequest {
		req := validCreateMK8SClusterRequest()
		req.Name = name
		return req
	}

	valid := map[string]string{
		"single letter":  "a",
		"single digit":   "1",
		"digits only":    "123",
		"leading digit":  "1demo",
		"inner hyphens":  "my-demo--cluster",
		"maximum length": strings.Repeat("a", MK8SClusterNameMaxLength),
	}
	for name, clusterName := range valid {
		t.Run("valid "+name, func(t *testing.T) {
			if err := withName(clusterName).Validate(); err != nil {
				t.Errorf("expected %q to be valid, got %v", clusterName, err)
			}
		})
	}

	invalid := map[string]string{
		"empty":                  "",
		"exceeds maximum length": strings.Repeat("a", MK8SClusterNameMaxLength+1),
		"63 characters":          strings.Repeat("a", 63),
		"uppercase":              "Demo",
		"underscore":             "my_cluster",
		"dot":                    "demo.prod",
		"leading hyphen":         "-demo",
		"trailing hyphen":        "demo-",
		"whitespace":             "my demo",
		"surrounding space":      " demo ",
		"non-ascii":              "démo",
		"64 characters":          strings.Repeat("a", 64),
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
