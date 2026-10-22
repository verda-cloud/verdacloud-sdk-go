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

//go:build integration && mk8s_local

package integration

// Contract test against a local managed Kubernetes API running on kind with no
// controllers. Nothing reconciles there, so clusters stay provisioning and
// admission-webhook rejections are not covered.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// Fixture values served by the local API's auth and network stubs
const (
	localAPIToken         = "local-session-token"
	localProjectID        = "11111111-1111-4111-8111-111111111111"
	localForeignProjectID = "22222222-2222-4222-8222-222222222222"
	localNetwork          = "pn_local"
	localForeignNetwork   = "pn_other_project"
	localSubnet           = "sn_local_fin01"
	localLocation         = "FIN-01"
)

// newLocalAPIClient returns a client whose BaseURL is a loopback shim.
// The shim answers the SDK's OAuth token request with the stub session token
// and forwards everything else to the local API's /public/v1 routes.
func newLocalAPIClient(t *testing.T) *verda.Client {
	t.Helper()

	raw := os.Getenv("VERDA_MK8S_LOCAL_URL")
	if raw == "" {
		t.Skip("VERDA_MK8S_LOCAL_URL must point at a local managed Kubernetes API, e.g. http://127.0.0.1:8080")
	}
	target, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid VERDA_MK8S_LOCAL_URL: %v", err)
	}
	if ip := net.ParseIP(target.Hostname()); target.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Fatalf("VERDA_MK8S_LOCAL_URL must be a loopback address, got %q", target.Host)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	shim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/oauth2/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"` + localAPIToken + `","token_type":"Bearer","expires_in":3600}`))
			return
		}
		r.URL.Path = "/public/v1" + r.URL.Path
		r.URL.RawPath = ""
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(shim.Close)

	client, err := verda.NewClient(
		verda.WithBaseURL(shim.URL),
		verda.WithClientID("local-client"),
		verda.WithClientSecret("local-secret"),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func localClusterName(t *testing.T) string {
	t.Helper()
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "sdk-local-" + hex.EncodeToString(b)
}

func localCreateRequest(name string) verda.CreateKubernetesClusterRequest {
	return verda.CreateKubernetesClusterRequest{
		Name:              name,
		KubernetesVersion: "1.35.8",
		PodCIDR:           "10.244.0.0/16",
		ServiceCIDR:       "10.96.0.0/12",
		Location:          localLocation,
		NetworkID:         localNetwork,
		SubnetID:          localSubnet,
	}
}

func requireAPIStatus(t *testing.T, err error, status int) {
	t.Helper()
	var apiErr *verda.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError with status %d, got %T: %v", status, err, err)
	}
	if apiErr.StatusCode != status {
		t.Fatalf("expected status %d, got %d: %s", status, apiErr.StatusCode, apiErr.Message)
	}
}

func TestKubernetesLocalClusterLifecycle(t *testing.T) {
	client := newLocalAPIClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	name := localClusterName(t)
	created, err := client.Kubernetes.Create(ctx, localProjectID, localCreateRequest(name))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Kubernetes.Delete(context.Background(), localProjectID, name) })

	if created.Name != name || created.Phase != verda.KubernetesClusterPhaseProvisioning {
		t.Errorf("unexpected created cluster %+v", created)
	}
	if created.NetworkID != localNetwork || created.SubnetID != localSubnet || created.Location != localLocation {
		t.Errorf("expected network/subnet/location echoed back, got %+v", created)
	}
	if created.ControlPlaneProfile != verda.KubernetesControlPlaneProfileStandard {
		t.Errorf("expected server-defaulted profile standard, got %q", created.ControlPlaneProfile)
	}

	got, err := client.Kubernetes.Get(ctx, localProjectID, name)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.KubernetesVersion != "1.35.8" || got.CreatedAt.IsZero() {
		t.Errorf("unexpected cluster %+v", got)
	}

	_, err = client.Kubernetes.Create(ctx, localProjectID, localCreateRequest(name))
	requireAPIStatus(t, err, http.StatusConflict)

	updated, err := client.Kubernetes.Update(ctx, localProjectID, name, verda.UpdateKubernetesClusterRequest{KubernetesVersion: "1.36.4"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.KubernetesVersion != "1.36.4" {
		t.Errorf("expected requested version 1.36.4, got %q", updated.KubernetesVersion)
	}

	if err := client.Kubernetes.Delete(ctx, localProjectID, name); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	// Without controllers there is no finalizer, so the object is gone at once
	_, err = client.Kubernetes.Get(ctx, localProjectID, name)
	requireAPIStatus(t, err, http.StatusNotFound)

	err = client.Kubernetes.Delete(ctx, localProjectID, name)
	requireAPIStatus(t, err, http.StatusNotFound)
}

func TestKubernetesLocalRejections(t *testing.T) {
	client := newLocalAPIClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Run("foreign network", func(t *testing.T) {
		req := localCreateRequest(localClusterName(t))
		req.NetworkID = localForeignNetwork
		_, err := client.Kubernetes.Create(ctx, localProjectID, req)
		requireAPIStatus(t, err, http.StatusBadRequest)
	})

	t.Run("subnet in another location", func(t *testing.T) {
		req := localCreateRequest(localClusterName(t))
		req.Location = "FIN-03"
		_, err := client.Kubernetes.Create(ctx, localProjectID, req)
		requireAPIStatus(t, err, http.StatusBadRequest)
	})

	t.Run("non-member project", func(t *testing.T) {
		_, err := client.Kubernetes.Get(ctx, localForeignProjectID, "anything")
		requireAPIStatus(t, err, http.StatusForbidden)
	})

	t.Run("missing cluster", func(t *testing.T) {
		_, err := client.Kubernetes.Get(ctx, localProjectID, localClusterName(t))
		requireAPIStatus(t, err, http.StatusNotFound)
	})

	t.Run("validation before HTTP", func(t *testing.T) {
		_, err := client.Kubernetes.Create(ctx, localProjectID, verda.CreateKubernetesClusterRequest{})
		var apiErr *verda.APIError
		if err == nil || errors.As(err, &apiErr) {
			t.Fatalf("expected a local validation error, got %v", err)
		}
	})
}
