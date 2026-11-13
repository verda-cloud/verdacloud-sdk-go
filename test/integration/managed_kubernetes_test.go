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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// Inputs of the managed Kubernetes lifecycle test.
const (
	mk8sKubernetesVersion = "1.36.4"
	mk8sUpgradeVersion    = "1.37.0"
	mk8sPodCIDR           = "10.244.0.0/16"
	mk8sServiceCIDR       = "10.96.0.0/12"
	mk8sKubeconfigCN      = "sdk-e2e"
	mk8sKubeconfigGroup   = "sdk-e2e-viewers"
)

// mk8sOptInEnv must be "1" for the tests that create or delete billable
// resources (the lifecycle test and the sweeper). Without it they skip, so a
// plain `make test-integration` does not create clusters.
const mk8sOptInEnv = "VERDA_E2E_MK8S"

// Time budgets, sized for `go test -timeout 17m`. Passing runs against
// production took 4.5 to 6 minutes. A `go test -timeout` panic does not run
// t.Cleanup, so the lifecycle test only starts when the test binary's
// deadline leaves room for the body and for every cleanup.
const (
	// mk8sLifecycleBudget caps the test body: the body context ends at the
	// earlier of start + mk8sLifecycleBudget and the deadline minus
	// mk8sCleanupReserve. Each step wait below is also capped on its own.
	mk8sLifecycleBudget = 9 * time.Minute
	// mk8sCleanupReserve is kept free after the body for the cleanups.
	mk8sCleanupReserve = 5*time.Minute + 30*time.Second
	// mk8sReportMargin is left after the cleanup window. It covers the
	// three final cleanups, which may each run mk8sCleanupStepMinimum past
	// the window, plus test reporting.
	mk8sReportMargin = 1*time.Minute + 45*time.Second

	mk8sWaitClusterRunning  = 4 * time.Minute
	mk8sWaitUpgrade         = 4 * time.Minute
	mk8sWaitPoolActive      = 5 * time.Minute
	mk8sWaitPoolScaledDown  = 3 * time.Minute
	mk8sWaitKubeconfigReady = 2 * time.Minute
	mk8sWaitPoolDeleted     = 3 * time.Minute
	mk8sWaitClusterDeleted  = 3 * time.Minute
	mk8sWaitKubeconfigGone  = 1 * time.Minute

	// Cleanup budgets. Each cleanup gets at most its own budget, and leaves
	// mk8sCleanupStepMinimum for every cleanup that runs after it.
	mk8sCleanupKubeconfig  = 1 * time.Minute
	mk8sCleanupPool        = 3 * time.Minute
	mk8sCleanupCluster     = 3 * time.Minute
	mk8sCleanupWorkers     = 2 * time.Minute
	mk8sCleanupStepMinimum = 30 * time.Second

	// mk8sCallTimeout bounds every API call; the default HTTP client has no
	// timeout, and a hung request must not use up the cleanup reserve.
	mk8sCallTimeout         = 60 * time.Second
	mk8sPollInterval        = 10 * time.Second
	mk8sDeleteRetryInterval = 10 * time.Second
)

// API error codes and reasons asserted by the tests.
const (
	mk8sCodeNotFound       = "not_found"
	mk8sCodeInvalidRequest = "invalid_request"
	mk8sCodeConflict       = "conflict"

	mk8sReasonClusterNotFound        = "cluster_not_found"
	mk8sReasonNodePoolNotFound       = "node_pool_not_found"
	mk8sReasonKubeconfigNotFound     = "kubeconfig_not_found"
	mk8sReasonAdmissionRejected      = "admission_rejected"
	mk8sReasonConcurrentModification = "concurrent_modification"
)

// errMk8sNoCapacity tells the caller to skip: the pool cannot get workers
// because of missing capacity or funds, which is not an SDK or API defect.
var errMk8sNoCapacity = errors.New("node pool cannot be provisioned")

// TestManagedKubernetesReadOnly checks the read-only endpoints and the 404
// errors of a missing cluster. It creates nothing.
func TestManagedKubernetesReadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := getTestClient(t)
	ctx := t.Context()

	enabled, err := call(ctx, client.ManagedKubernetes.GetEnabledLocations)
	if err != nil {
		t.Fatalf("GetEnabledLocations: %v", err)
	}
	if !sort.StringsAreSorted(enabled.Locations) {
		t.Errorf("enabled locations are not sorted: %v", enabled.Locations)
	}
	t.Logf("enabled locations: %v", enabled.Locations)

	clusters, err := call(ctx, client.ManagedKubernetes.List)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	t.Logf("project has %d managed Kubernetes clusters", len(clusters))

	if len(enabled.Locations) == 0 {
		t.Skip("no location is enabled for managed Kubernetes; skipping the not-found checks")
	}
	location := enabled.Locations[0]
	missing := "sdk-e2e-" + randomHex(t)

	_, err = call(ctx, func(c context.Context) (*verda.ManagedKubernetesCluster, error) {
		return client.ManagedKubernetes.Get(c, location, missing)
	})
	requireAPIError(t, err, http.StatusNotFound, mk8sCodeNotFound, mk8sReasonClusterNotFound)

	_, err = call(ctx, func(c context.Context) ([]verda.ManagedKubernetesNodePool, error) {
		return client.ManagedKubernetes.ListNodePools(c, location, missing)
	})
	requireAPIError(t, err, http.StatusNotFound, mk8sCodeNotFound, mk8sReasonClusterNotFound)

	_, err = call(ctx, func(c context.Context) (*verda.ManagedKubernetesKubeconfig, error) {
		return client.ManagedKubernetes.GetKubeconfig(c, location, missing, missingKubeconfigID(t, missing))
	})
	requireAPIError(t, err, http.StatusNotFound, mk8sCodeNotFound, mk8sReasonClusterNotFound)
}

// TestManagedKubernetesLifecycle creates a cluster with one node pool and a
// kubeconfig, upgrades and scales them, and deletes everything. It creates
// billable resources and runs only with VERDA_E2E_MK8S=1; run it with
// `make test-integration-mk8s`.
func TestManagedKubernetesLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}
	requireMk8sOptIn(t)

	// The deadline check runs before getTestClient and before any API call.
	// The test is opted in, so a deadline that is too short is a failure and
	// not a skip: a skip would let the run pass without testing anything.
	// Without a deadline (-timeout 0) no timeout panic can skip the cleanups,
	// and the budgets are measured from now.
	required := mk8sLifecycleBudget + mk8sCleanupReserve
	deadline, hasDeadline := t.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(required)
	} else if remaining := time.Until(deadline); remaining < required {
		t.Fatalf("needs at least %s before the test deadline, has %s; run with -timeout 17m or more, e.g. make test-integration-mk8s", required, remaining.Round(time.Second))
	}

	client := getTestClient(t)

	bodyEnd := time.Now().Add(mk8sLifecycleBudget)
	if reserveStart := deadline.Add(-mk8sCleanupReserve); reserveStart.Before(bodyEnd) {
		bodyEnd = reserveStart
	}
	bodyCtx, cancelBody := context.WithDeadline(t.Context(), bodyEnd)
	defer cancelBody()
	budget := cleanupBudget{end: deadline.Add(-mk8sReportMargin)}

	suffix := randomHex(t)
	clusterName := "sdk-e2e-" + suffix
	poolName := "pool-" + suffix
	step := func(name string) { t.Logf("=== step: %s", name) }

	step("select location and instance type")
	location, instanceType := selectMk8sLocation(bodyCtx, t, client)
	t.Logf("location %s, instance type %s", location, instanceType)

	step("create SSH key, network and subnet")
	sshKeyID := createEphemeralSSHKey(bodyCtx, t, client, budget, "sdk-e2e-"+suffix)
	network, subnet := createMk8sNetwork(bodyCtx, t, client, budget, location, suffix)
	registerWorkerCheck(t, client, budget, subnet.ID)

	// Cluster: register the cleanup before the POST, so that an ambiguous
	// create failure is cleaned up too.
	step("create cluster")
	t.Cleanup(func() {
		ctx, cancel := budget.context(mk8sCleanupCluster, 2)
		defer cancel()
		if err := deleteClusterAndWait(ctx, t, client, location, clusterName, true); err != nil {
			t.Errorf("cleanup: cluster %s/%s: %v", location, clusterName, err)
		}
	})
	createReq := verda.CreateManagedKubernetesClusterRequest{
		Name:              clusterName,
		KubernetesVersion: mk8sKubernetesVersion,
		PodCIDR:           mk8sPodCIDR,
		ServiceCIDR:       mk8sServiceCIDR,
		Location:          location,
		NetworkID:         network.ID,
		SubnetID:          subnet.ID,
	}
	created, err := call(bodyCtx, func(c context.Context) (*verda.ManagedKubernetesCluster, error) {
		return client.ManagedKubernetes.Create(c, createReq)
	})
	if err != nil {
		t.Fatalf("Create cluster %s: %v", clusterName, err)
	}
	assertClusterSpec(t, "create response", created, createReq, mk8sKubernetesVersion)
	if created.Phase != verda.ManagedKubernetesClusterPhaseProvisioning && created.Phase != verda.ManagedKubernetesClusterPhaseRunning {
		t.Errorf("create response phase = %q, want provisioning or running", created.Phase)
	}

	step("get and list cluster")
	got, err := call(bodyCtx, func(c context.Context) (*verda.ManagedKubernetesCluster, error) {
		return client.ManagedKubernetes.Get(c, location, clusterName)
	})
	if err != nil {
		t.Fatalf("Get cluster: %v", err)
	}
	assertClusterSpec(t, "get", got, createReq, mk8sKubernetesVersion)
	clusters, err := call(bodyCtx, client.ManagedKubernetes.List)
	if err != nil {
		t.Fatalf("List clusters: %v", err)
	}
	if !slices.ContainsFunc(clusters, func(c verda.ManagedKubernetesCluster) bool {
		return c.Name == clusterName && c.Location == location
	}) {
		t.Errorf("List does not contain cluster %s/%s", location, clusterName)
	}

	step("wait for cluster running")
	running := waitForCluster(bodyCtx, t, client, location, clusterName, mk8sWaitClusterRunning, func(c *verda.ManagedKubernetesCluster) bool {
		return c.Phase == verda.ManagedKubernetesClusterPhaseRunning && c.ExternalEndpoint != ""
	})

	step("missing children of an existing cluster")
	_, err = call(bodyCtx, func(c context.Context) (*verda.ManagedKubernetesNodePool, error) {
		return client.ManagedKubernetes.GetNodePool(c, location, clusterName, "missing-"+randomHex(t))
	})
	requireAPIError(t, err, http.StatusNotFound, mk8sCodeNotFound, mk8sReasonNodePoolNotFound)
	_, err = call(bodyCtx, func(c context.Context) (*verda.ManagedKubernetesKubeconfig, error) {
		return client.ManagedKubernetes.GetKubeconfig(c, location, clusterName, missingKubeconfigID(t, clusterName))
	})
	requireAPIError(t, err, http.StatusNotFound, mk8sCodeNotFound, mk8sReasonKubeconfigNotFound)

	step("reject unknown version")
	_, err = updateClusterVersion(bodyCtx, client, location, clusterName, "0.0.1")
	requireAPIError(t, err, http.StatusBadRequest, mk8sCodeInvalidRequest, mk8sReasonAdmissionRejected)

	// The upgrade step checks that the API accepts and persists the new
	// version, and that the cluster reports running at that version. The
	// phase can lag the spec, so this does not prove that the rollout
	// finished. It runs before any workers exist; worker upgrades are out of
	// scope.
	step("upgrade cluster")
	upgraded, err := updateClusterVersion(bodyCtx, client, location, clusterName, mk8sUpgradeVersion)
	if err != nil {
		t.Fatalf("upgrade to %s: %v", mk8sUpgradeVersion, err)
	}
	if upgraded.KubernetesVersion != mk8sUpgradeVersion {
		t.Fatalf("upgrade response kubernetes_version = %q, want %q", upgraded.KubernetesVersion, mk8sUpgradeVersion)
	}
	running = waitForCluster(bodyCtx, t, client, location, clusterName, mk8sWaitUpgrade, func(c *verda.ManagedKubernetesCluster) bool {
		return c.Phase == verda.ManagedKubernetesClusterPhaseRunning && c.KubernetesVersion == mk8sUpgradeVersion && c.ExternalEndpoint != ""
	})

	step("reject downgrade")
	_, err = updateClusterVersion(bodyCtx, client, location, clusterName, mk8sKubernetesVersion)
	requireAPIError(t, err, http.StatusBadRequest, mk8sCodeInvalidRequest, mk8sReasonAdmissionRejected)

	// Node pool: register the cleanup before the POST.
	step("create node pool")
	t.Cleanup(func() {
		ctx, cancel := budget.context(mk8sCleanupPool, 3)
		defer cancel()
		if err := deleteNodePoolAndWait(ctx, t, client, location, clusterName, poolName, true); err != nil {
			t.Errorf("cleanup: node pool %s/%s/%s: %v", location, clusterName, poolName, err)
		}
	})
	replicas := 1
	poolReq := verda.CreateManagedKubernetesNodePoolRequest{
		Name:         poolName,
		Replicas:     &replicas,
		InstanceType: instanceType,
		SSHKeyIDs:    []string{sshKeyID},
	}
	pool, err := call(bodyCtx, func(c context.Context) (*verda.ManagedKubernetesNodePool, error) {
		return client.ManagedKubernetes.CreateNodePool(c, location, clusterName, poolReq)
	})
	if err != nil {
		t.Fatalf("CreateNodePool %s: %v", poolName, err)
	}
	assertNodePoolSpec(t, "create response", pool, poolReq, replicas)

	step("wait for node pool active")
	err = waitForNodePool(bodyCtx, t, client, location, clusterName, poolName, mk8sWaitPoolActive, func(p *verda.ManagedKubernetesNodePool) (bool, error) {
		if p.State == verda.ManagedKubernetesNodePoolStateDegraded && len(p.Errors) > 0 {
			return false, nodePoolError(p)
		}
		return p.State == verda.ManagedKubernetesNodePoolStateActive && p.ReadyReplicas == replicas, nil
	})
	if errors.Is(err, errMk8sNoCapacity) {
		t.Skipf("skipping the rest of the lifecycle: %v", err)
	}
	if err != nil {
		t.Fatalf("node pool %s did not become active: %v", poolName, err)
	}

	// Positive control for the worker check: the teardown check only means
	// something if the pool's workers show up as instances in the subnet.
	step("workers visible in subnet")
	if workers := instancesInSubnet(bodyCtx, t, client, subnet.ID); len(workers) < replicas {
		t.Errorf("found %d instances in subnet %s while the pool has %d ready replicas; the worker teardown check cannot see the workers", len(workers), subnet.ID, replicas)
	}

	step("get and list node pools")
	pool, err = call(bodyCtx, func(c context.Context) (*verda.ManagedKubernetesNodePool, error) {
		return client.ManagedKubernetes.GetNodePool(c, location, clusterName, poolName)
	})
	if err != nil {
		t.Fatalf("GetNodePool: %v", err)
	}
	assertNodePoolSpec(t, "get", pool, poolReq, replicas)
	pools, err := call(bodyCtx, func(c context.Context) ([]verda.ManagedKubernetesNodePool, error) {
		return client.ManagedKubernetes.ListNodePools(c, location, clusterName)
	})
	if err != nil {
		t.Fatalf("ListNodePools: %v", err)
	}
	if !slices.ContainsFunc(pools, func(p verda.ManagedKubernetesNodePool) bool { return p.Name == poolName }) {
		t.Errorf("ListNodePools does not contain %s", poolName)
	}

	// Scaling to zero proves that the desired count changed. It does not
	// prove that the worker VMs are gone; the worker check covers that.
	step("scale node pool to zero")
	zero := 0
	scaled, err := call(bodyCtx, func(c context.Context) (*verda.ManagedKubernetesNodePool, error) {
		return client.ManagedKubernetes.UpdateNodePool(c, location, clusterName, poolName, verda.UpdateManagedKubernetesNodePoolRequest{Replicas: &zero})
	})
	if err != nil {
		t.Fatalf("UpdateNodePool replicas=0: %v", err)
	}
	if scaled.Replicas != 0 {
		t.Errorf("UpdateNodePool response replicas = %d, want 0", scaled.Replicas)
	}
	err = waitForNodePool(bodyCtx, t, client, location, clusterName, poolName, mk8sWaitPoolScaledDown, func(p *verda.ManagedKubernetesNodePool) (bool, error) {
		return p.Replicas == 0 && p.ReadyReplicas == 0, nil
	})
	if err != nil {
		t.Fatalf("node pool %s did not scale to zero: %v", poolName, err)
	}

	step("kubeconfig")
	exerciseKubeconfig(bodyCtx, t, client, budget, location, clusterName, running.ExternalEndpoint)

	step("delete node pool")
	if err := deleteNodePoolAndWait(bodyCtx, t, client, location, clusterName, poolName, false); err != nil {
		t.Fatalf("delete node pool %s: %v", poolName, err)
	}

	step("delete cluster")
	if err := deleteClusterAndWait(bodyCtx, t, client, location, clusterName, false); err != nil {
		t.Fatalf("delete cluster %s: %v", clusterName, err)
	}
	t.Log("lifecycle complete; cleanups verify that no workers or networks remain")
}

// exerciseKubeconfig creates a kubeconfig, waits until it is ready, checks
// the list and deletes it. It never logs the kubeconfig YAML.
func exerciseKubeconfig(ctx context.Context, t *testing.T, client *verda.Client, budget cleanupBudget, location, clusterName, endpoint string) {
	t.Helper()

	kc, err := call(ctx, func(c context.Context) (*verda.ManagedKubernetesKubeconfig, error) {
		return client.ManagedKubernetes.CreateKubeconfig(c, location, clusterName, verda.CreateManagedKubernetesKubeconfigRequest{
			CommonName: mk8sKubeconfigCN,
			Groups:     []string{mk8sKubeconfigGroup},
			Duration:   "1h",
		})
	})
	// Register the cleanup before any assertion. When the POST failed, the
	// cleanup finds the kubeconfig by its common name; the cluster belongs to
	// this run only.
	createdID := ""
	if err == nil && kc != nil {
		createdID = kc.ID
	}
	t.Cleanup(func() {
		cctx, cancel := budget.context(mk8sCleanupKubeconfig, 4)
		defer cancel()
		cleanupKubeconfigs(cctx, t, client, location, clusterName, createdID)
	})
	if err != nil {
		t.Fatalf("CreateKubeconfig: %v", err)
	}
	if kc.ID == "" {
		t.Fatal("CreateKubeconfig returned an empty id")
	}
	if kc.Status != verda.ManagedKubernetesKubeconfigStatusPending && kc.Status != verda.ManagedKubernetesKubeconfigStatusReady {
		t.Errorf("CreateKubeconfig status = %q, want pending or ready", kc.Status)
	}

	var ready *verda.ManagedKubernetesKubeconfig
	err = waitFor(ctx, t, "kubeconfig "+kc.ID+" ready", mk8sWaitKubeconfigReady, func(c context.Context) (bool, string, error) {
		got, err := client.ManagedKubernetes.GetKubeconfig(c, location, clusterName, kc.ID)
		if err != nil {
			return false, "", err
		}
		ready = got
		return got.Status == verda.ManagedKubernetesKubeconfigStatusReady, "status=" + got.Status, nil
	})
	if err != nil {
		t.Fatalf("kubeconfig %s did not become ready: %v", kc.ID, err)
	}
	if ready.NotAfter == nil {
		t.Error("ready kubeconfig has no not_after")
	}
	// Booleans only: failure messages must never contain the YAML.
	if ready.Kubeconfig == "" {
		t.Error("ready kubeconfig has no YAML")
	}
	if endpoint == "" {
		t.Error("cluster has no external endpoint to compare the kubeconfig with")
	} else if !strings.Contains(ready.Kubeconfig, endpoint) {
		t.Errorf("kubeconfig YAML does not contain the cluster's external endpoint %s", endpoint)
	}

	list, err := call(ctx, func(c context.Context) ([]verda.ManagedKubernetesKubeconfig, error) {
		return client.ManagedKubernetes.ListKubeconfigs(c, location, clusterName)
	})
	if err != nil {
		t.Fatalf("ListKubeconfigs: %v", err)
	}
	idx := slices.IndexFunc(list, func(k verda.ManagedKubernetesKubeconfig) bool { return k.ID == kc.ID })
	if idx < 0 {
		t.Errorf("ListKubeconfigs does not contain %s", kc.ID)
	} else if list[idx].Kubeconfig != "" {
		t.Error("ListKubeconfigs returned kubeconfig YAML; the list must carry metadata only")
	}

	err = callErr(ctx, func(c context.Context) error {
		return client.ManagedKubernetes.DeleteKubeconfig(c, location, clusterName, kc.ID)
	})
	if err != nil {
		t.Fatalf("DeleteKubeconfig %s: %v", kc.ID, err)
	}
	err = waitFor(ctx, t, "kubeconfig "+kc.ID+" deleted", mk8sWaitKubeconfigGone, func(c context.Context) (bool, string, error) {
		got, err := client.ManagedKubernetes.GetKubeconfig(c, location, clusterName, kc.ID)
		if err == nil {
			return false, "status=" + got.Status, nil
		}
		if isAPIError(err, http.StatusNotFound, mk8sReasonKubeconfigNotFound) {
			return true, "gone", nil
		}
		return false, "", err
	})
	if err != nil {
		t.Fatalf("kubeconfig %s was not deleted: %v", kc.ID, err)
	}
}

// cleanupKubeconfigs deletes the run's kubeconfig. Without an id (the POST
// failed) it deletes every kubeconfig of the cluster with the test's common
// name.
func cleanupKubeconfigs(ctx context.Context, t *testing.T, client *verda.Client, location, clusterName, id string) {
	t.Helper()

	ids := []string{id}
	if id == "" {
		list, err := call(ctx, func(c context.Context) ([]verda.ManagedKubernetesKubeconfig, error) {
			return client.ManagedKubernetes.ListKubeconfigs(c, location, clusterName)
		})
		if isAPIError(err, http.StatusNotFound, mk8sReasonClusterNotFound) {
			return
		}
		if err != nil {
			t.Errorf("cleanup: list kubeconfigs of %s/%s: %v", location, clusterName, err)
			return
		}
		ids = nil
		for _, k := range list {
			if k.CommonName == mk8sKubeconfigCN {
				ids = append(ids, k.ID)
			}
		}
	}
	for _, kcID := range ids {
		_, err := deleteWithRetry(ctx, t, "kubeconfig "+kcID+" of "+location+"/"+clusterName, true, func(c context.Context) error {
			return client.ManagedKubernetes.DeleteKubeconfig(c, location, clusterName, kcID)
		}, func(err error) bool {
			return isAPIError(err, http.StatusNotFound, mk8sReasonKubeconfigNotFound, mk8sReasonClusterNotFound)
		})
		if err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
}

// selectMk8sLocation returns a location that is enabled for managed
// Kubernetes and supports private networks, with the cheapest on-demand
// instance type available there. It skips when there is none.
func selectMk8sLocation(ctx context.Context, t *testing.T, client *verda.Client) (location, instanceType string) {
	t.Helper()

	enabled, err := call(ctx, client.ManagedKubernetes.GetEnabledLocations)
	if err != nil {
		t.Fatalf("GetEnabledLocations: %v", err)
	}
	locations, err := call(ctx, client.Locations.Get)
	if err != nil {
		t.Fatalf("Locations.Get: %v", err)
	}
	privateNetworks := make(map[string]bool, len(locations))
	for _, loc := range locations {
		privateNetworks[loc.Code] = loc.IsPrivateNetworksEnabled
	}

	for _, code := range enabled.Locations {
		if !privateNetworks[code] {
			continue
		}
		if it, ok := findCheapestOnDemandTypeIn(ctx, t, client, code); ok {
			return code, it
		}
		t.Logf("no on-demand instance type is available in %s", code)
	}
	t.Skipf("no location is enabled for managed Kubernetes, supports private networks and has an available on-demand instance type (enabled: %v)", enabled.Locations)
	return "", ""
}

// findCheapestOnDemandTypeIn returns the available on-demand instance type
// with the lowest price_per_hour in location. API errors fail the test;
// only an empty result reports false.
func findCheapestOnDemandTypeIn(ctx context.Context, t *testing.T, client *verda.Client, location string) (string, bool) {
	t.Helper()

	availabilities, err := call(ctx, func(c context.Context) ([]verda.LocationAvailability, error) {
		return client.InstanceAvailability.GetAllAvailabilities(c, false, location)
	})
	if err != nil {
		t.Fatalf("GetAllAvailabilities(%s): %v", location, err)
	}
	available := map[string]bool{}
	for _, a := range availabilities {
		if a.LocationCode != location {
			continue
		}
		for _, it := range a.Availabilities {
			available[it] = true
		}
	}

	types, err := call(ctx, func(c context.Context) ([]verda.InstanceTypeInfo, error) {
		return client.InstanceTypes.Get(c, "usd")
	})
	if err != nil {
		t.Fatalf("InstanceTypes.Get: %v", err)
	}
	// A zero price means the type has no on-demand price, not that it is free.
	types = slices.DeleteFunc(types, func(it verda.InstanceTypeInfo) bool {
		return !available[it.InstanceType] || it.PricePerHour <= 0
	})
	if len(types) == 0 {
		return "", false
	}
	slices.SortFunc(types, func(a, b verda.InstanceTypeInfo) int {
		if a.PricePerHour != b.PricePerHour {
			if a.PricePerHour < b.PricePerHour {
				return -1
			}
			return 1
		}
		return strings.Compare(a.InstanceType, b.InstanceType)
	})
	t.Logf("cheapest on-demand type in %s: %s ($%.4f/h)", location, types[0].InstanceType, float64(types[0].PricePerHour))
	return types[0].InstanceType, true
}

// createEphemeralSSHKey registers a freshly generated ed25519 public key and
// deletes it in a cleanup. The private key is discarded; nobody logs in to
// the workers.
func createEphemeralSSHKey(ctx context.Context, t *testing.T, client *verda.Client, budget cleanupBudget, name string) string {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	key, err := call(ctx, func(c context.Context) (*verda.SSHKey, error) {
		return client.SSHKeys.AddSSHKey(c, &verda.CreateSSHKeyRequest{Name: name, PublicKey: ed25519AuthorizedKey(pub)})
	})
	if err != nil {
		t.Fatalf("AddSSHKey %s: %v", name, err)
	}
	t.Logf("created SSH key %s (%s)", key.ID, name)
	t.Cleanup(func() {
		cctx, cancel := budget.finalContext()
		defer cancel()
		if err := deleteSSHKey(cctx, t, client, key.ID, name); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return key.ID
}

// ed25519AuthorizedKey encodes pub in the OpenSSH authorized_keys format
// (RFC 8709, section 4): "ssh-ed25519 " followed by the base64 of
// string("ssh-ed25519") || string(key), each string prefixed with its
// big-endian uint32 length.
func ed25519AuthorizedKey(pub ed25519.PublicKey) string {
	const keyType = "ssh-ed25519"
	blob := make([]byte, 0, 4+len(keyType)+4+ed25519.PublicKeySize)
	blob = binary.BigEndian.AppendUint32(blob, uint32(len(keyType)))
	blob = append(blob, keyType...)
	blob = binary.BigEndian.AppendUint32(blob, uint32(ed25519.PublicKeySize))
	blob = append(blob, pub...)
	return keyType + " " + base64.StdEncoding.EncodeToString(blob)
}

// createMk8sNetwork creates a custom-mode private network and a random
// 10.(200-239).x.0/24 subnet in location, which overlaps neither the pod nor
// the service CIDR. Both are deleted in cleanups.
func createMk8sNetwork(ctx context.Context, t *testing.T, client *verda.Client, budget cleanupBudget, location, suffix string) (*verda.PrivateNetwork, *verda.Subnet) {
	t.Helper()

	network, err := call(ctx, func(c context.Context) (*verda.PrivateNetwork, error) {
		return client.PrivateNetworks.Create(c, verda.CreatePrivateNetworkRequest{
			Name: "sdk-e2e-net-" + suffix,
			Mode: verda.NetworkModeCustom,
		})
	})
	if err != nil {
		t.Fatalf("create private network: %v", err)
	}
	t.Logf("created private network %s (%s)", network.ID, network.Name)
	t.Cleanup(func() {
		cctx, cancel := budget.finalContext()
		defer cancel()
		if err := deleteNetwork(cctx, t, client, network); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	cidr := fmt.Sprintf("10.%d.%d.0/24", 200+mathrand.IntN(40), mathrand.IntN(256))
	subnet, err := call(ctx, func(c context.Context) (*verda.Subnet, error) {
		return client.PrivateNetworks.CreateSubnet(c, network.ID, verda.CreateSubnetRequest{
			LocationCode: location,
			Name:         "sdk-e2e-subnet-" + suffix,
			CIDR:         cidr,
		})
	})
	if err != nil {
		t.Fatalf("create subnet %s in %s: %v", cidr, location, err)
	}
	t.Logf("created subnet %s (%s) in %s", subnet.ID, subnet.CIDR, location)
	t.Cleanup(func() {
		cctx, cancel := budget.finalContext()
		defer cancel()
		if err := deleteSubnet(cctx, t, client, network.ID, subnet.ID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return network, subnet
}

// deleteSSHKey, deleteNetwork and deleteSubnet are used by the cleanups and
// the sweeper. They retry DELETE like deleteWithRetry, and a 404 counts as
// already deleted, so that a parallel sweep does not fail the cleanup.
func deleteSSHKey(ctx context.Context, t *testing.T, client *verda.Client, id, name string) error {
	t.Helper()
	_, err := deleteWithRetry(ctx, t, "SSH key "+id+" ("+name+")", true, func(c context.Context) error {
		return client.SSHKeys.DeleteSSHKey(c, id)
	}, isNotFound)
	return err
}

func deleteNetwork(ctx context.Context, t *testing.T, client *verda.Client, network *verda.PrivateNetwork) error {
	t.Helper()
	_, err := deleteWithRetry(ctx, t, "private network "+network.ID+" ("+network.Name+")", true, func(c context.Context) error {
		return client.PrivateNetworks.Delete(c, network.ID)
	}, isNotFound)
	return err
}

func deleteSubnet(ctx context.Context, t *testing.T, client *verda.Client, networkID, subnetID string) error {
	t.Helper()
	_, err := deleteWithRetry(ctx, t, "subnet "+subnetID+" of network "+networkID, true, func(c context.Context) error {
		return client.PrivateNetworks.DeleteSubnet(c, networkID, subnetID)
	}, isNotFound)
	return err
}

// instancesInSubnet returns the instances in subnetID that are not
// discontinued.
func instancesInSubnet(ctx context.Context, t *testing.T, client *verda.Client, subnetID string) []string {
	t.Helper()

	instances, err := call(ctx, func(c context.Context) ([]verda.Instance, error) { return client.Instances.List(c, nil) })
	if err != nil {
		t.Fatalf("Instances.List: %v", err)
	}
	return filterInstancesInSubnet(instances, subnetID)
}

// filterInstancesInSubnet returns "<id> (<status>)" for every instance in
// subnetID. A discontinued instance is deleted and no longer billed, so it
// is left out.
func filterInstancesInSubnet(instances []verda.Instance, subnetID string) []string {
	var found []string
	for _, inst := range instances {
		if inst.SubnetID != nil && *inst.SubnetID == subnetID && inst.Status != verda.StatusDiscontinued {
			found = append(found, inst.ID+" ("+inst.Status+")")
		}
	}
	return found
}

// registerWorkerCheck registers a cleanup that runs after the cluster and
// pool cleanups and before the subnet delete. A deleted pool does not prove
// that its worker VMs are gone, so the check waits until no instance remains
// in the test subnet and fails the test with the leftover instance IDs.
func registerWorkerCheck(t *testing.T, client *verda.Client, budget cleanupBudget, subnetID string) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := budget.context(mk8sCleanupWorkers, 1)
		defer cancel()
		var leftover []string
		var listErr error
		listed := false
		err := waitFor(ctx, t, "no instances in subnet "+subnetID, mk8sCleanupWorkers, func(c context.Context) (bool, string, error) {
			instances, err := client.Instances.List(c, nil)
			if err != nil {
				// Keep polling: one failed list must not end the check.
				listErr = err
				return false, "list failed", nil
			}
			listed = true
			leftover = filterInstancesInSubnet(instances, subnetID)
			return len(leftover) == 0, fmt.Sprintf("%d instances", len(leftover)), nil
		})
		switch {
		case err == nil:
		case !listed:
			t.Errorf("cleanup: could not verify that no worker instances remain in subnet %s: %v; last list error: %v", subnetID, err, listErr)
		default:
			t.Errorf("cleanup: worker instances remain in subnet %s: %v; instances: %v", subnetID, err, leftover)
		}
	})
}

// deleteClusterAndWait deletes the cluster and waits until it returns 404.
// In a cleanup (inCleanup) an already missing cluster counts as done, any
// 200 state is accepted while waiting, and a transient read error (see
// isTransient) is polled again. In the test body the DELETE must be
// accepted, and every read until the 404 must show the deleting phase.
func deleteClusterAndWait(ctx context.Context, t *testing.T, client *verda.Client, location, clusterName string, inCleanup bool) error {
	t.Helper()

	what := "cluster " + location + "/" + clusterName
	gone, err := deleteWithRetry(ctx, t, what, inCleanup, func(c context.Context) error {
		return client.ManagedKubernetes.Delete(c, location, clusterName)
	}, func(err error) bool {
		return isAPIError(err, http.StatusNotFound, mk8sReasonClusterNotFound)
	})
	if err != nil {
		return err
	}
	if gone {
		if inCleanup {
			return nil
		}
		return fmt.Errorf("%s was already gone before DELETE", what)
	}
	timeout := mk8sWaitClusterDeleted
	if inCleanup {
		timeout = mk8sCleanupCluster
	}
	return waitFor(ctx, t, what+" deleted", timeout, func(c context.Context) (bool, string, error) {
		cluster, err := client.ManagedKubernetes.Get(c, location, clusterName)
		if err == nil {
			if !inCleanup && cluster.Phase != verda.ManagedKubernetesClusterPhaseDeleting {
				return false, "", fmt.Errorf("phase after DELETE = %q, want %q", cluster.Phase, verda.ManagedKubernetesClusterPhaseDeleting)
			}
			return false, "phase=" + cluster.Phase, nil
		}
		if isAPIError(err, http.StatusNotFound, mk8sReasonClusterNotFound) {
			return true, "gone", nil
		}
		if inCleanup && isTransient(err) {
			// Keep polling: one failed read must not end a cleanup wait.
			return false, "read failed", nil
		}
		return false, "", err
	})
}

// deleteNodePoolAndWait deletes the pool and waits until it returns 404
// node_pool_not_found. In a cleanup (inCleanup) an already missing pool or
// cluster counts as done, any 200 state is accepted while waiting, and a
// transient read error (see isTransient) is polled again. In the test body
// the DELETE must be accepted, the cluster must still exist, and every read
// until the 404 must show the deleting state.
func deleteNodePoolAndWait(ctx context.Context, t *testing.T, client *verda.Client, location, clusterName, poolName string, inCleanup bool) error {
	t.Helper()

	what := "node pool " + location + "/" + clusterName + "/" + poolName
	poolGone := func(err error) bool {
		if inCleanup {
			return isAPIError(err, http.StatusNotFound, mk8sReasonNodePoolNotFound, mk8sReasonClusterNotFound)
		}
		return isAPIError(err, http.StatusNotFound, mk8sReasonNodePoolNotFound)
	}
	gone, err := deleteWithRetry(ctx, t, what, inCleanup, func(c context.Context) error {
		return client.ManagedKubernetes.DeleteNodePool(c, location, clusterName, poolName)
	}, poolGone)
	if err != nil {
		return err
	}
	if gone {
		if inCleanup {
			return nil
		}
		return fmt.Errorf("%s was already gone before DELETE", what)
	}
	timeout := mk8sWaitPoolDeleted
	if inCleanup {
		timeout = mk8sCleanupPool
	}
	return waitFor(ctx, t, what+" deleted", timeout, func(c context.Context) (bool, string, error) {
		pool, err := client.ManagedKubernetes.GetNodePool(c, location, clusterName, poolName)
		if err == nil {
			if !inCleanup && pool.State != verda.ManagedKubernetesNodePoolStateDeleting {
				return false, "", fmt.Errorf("state after DELETE = %q, want %q", pool.State, verda.ManagedKubernetesNodePoolStateDeleting)
			}
			return false, "state=" + pool.State, nil
		}
		if poolGone(err) {
			return true, "gone", nil
		}
		if inCleanup && isTransient(err) {
			// Keep polling: one failed read must not end a cleanup wait.
			return false, "read failed", nil
		}
		return false, "", err
	})
}

// deleteWithRetry sends DELETE until the API accepts it. A 409
// concurrent_modification means the deletion was not accepted, so it is
// retried every mk8sDeleteRetryInterval until ctx ends. With retryTransient
// (cleanups), a transient error (see isTransient) is retried the same way,
// so that it does not leak a billable resource. It reports gone when
// the error says the resource does not exist.
func deleteWithRetry(ctx context.Context, t *testing.T, what string, retryTransient bool, del func(context.Context) error, isGone func(error) bool) (bool, error) {
	t.Helper()

	for {
		delErr := callErr(ctx, del)
		switch {
		case delErr == nil:
			t.Logf("DELETE %s accepted", what)
			return false, nil
		case isGone(delErr):
			return true, nil
		case isAPIError(delErr, http.StatusConflict, mk8sReasonConcurrentModification) && apiErrorCode(delErr) == mk8sCodeConflict:
			t.Logf("DELETE %s: concurrent modification, retrying in %s", what, mk8sDeleteRetryInterval)
		case retryTransient && ctx.Err() == nil && isTransient(delErr):
			t.Logf("DELETE %s: %v; retrying in %s", what, delErr, mk8sDeleteRetryInterval)
		default:
			return false, fmt.Errorf("DELETE %s: %w", what, delErr)
		}
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("DELETE %s not accepted before the deadline: %w", what, delErr)
		case <-time.After(mk8sDeleteRetryInterval):
		}
	}
}

// waitForCluster polls the cluster until done reports true and returns the
// last read. On timeout the failure lists the cluster's conditions.
func waitForCluster(ctx context.Context, t *testing.T, client *verda.Client, location, clusterName string, timeout time.Duration, done func(*verda.ManagedKubernetesCluster) bool) *verda.ManagedKubernetesCluster {
	t.Helper()

	var last *verda.ManagedKubernetesCluster
	err := waitFor(ctx, t, "cluster "+clusterName, timeout, func(c context.Context) (bool, string, error) {
		cluster, err := client.ManagedKubernetes.Get(c, location, clusterName)
		if err != nil {
			return false, "", err
		}
		last = cluster
		return done(cluster), fmt.Sprintf("phase=%s version=%s endpoint=%t", cluster.Phase, cluster.KubernetesVersion, cluster.ExternalEndpoint != ""), nil
	})
	if err != nil {
		conditions := "none"
		if last != nil {
			conditions = describeConditions(last.Conditions)
		}
		t.Fatalf("cluster %s: %v; conditions: %s", clusterName, err, conditions)
	}
	return last
}

// waitForNodePool polls the pool until done reports true or an error.
func waitForNodePool(ctx context.Context, t *testing.T, client *verda.Client, location, clusterName, poolName string, timeout time.Duration, done func(*verda.ManagedKubernetesNodePool) (bool, error)) error {
	t.Helper()

	return waitFor(ctx, t, "node pool "+poolName, timeout, func(c context.Context) (bool, string, error) {
		pool, err := client.ManagedKubernetes.GetNodePool(c, location, clusterName, poolName)
		if err != nil {
			return false, "", err
		}
		ok, err := done(pool)
		return ok, fmt.Sprintf("state=%s replicas=%d ready=%d", pool.State, pool.Replicas, pool.ReadyReplicas), err
	})
}

// nodePoolError turns the errors of a degraded pool into a test error.
// Missing capacity or funds wraps errMk8sNoCapacity.
func nodePoolError(pool *verda.ManagedKubernetesNodePool) error {
	parts := make([]string, 0, len(pool.Errors))
	noCapacity := false
	for _, e := range pool.Errors {
		parts = append(parts, e.Reason+": "+e.Message)
		if e.Reason == verda.ManagedKubernetesNodePoolErrorReasonInsufficientCapacity || e.Reason == verda.ManagedKubernetesNodePoolErrorReasonInsufficientFunds {
			noCapacity = true
		}
	}
	if noCapacity {
		return fmt.Errorf("%w: %s", errMk8sNoCapacity, strings.Join(parts, "; "))
	}
	return fmt.Errorf("node pool %s is degraded: %s", pool.Name, strings.Join(parts, "; "))
}

// waitFor polls check every mk8sPollInterval until it reports done, returns
// an error, or the timeout or ctx ends. Each check gets its own call timeout.
// It logs every state change and names the last state on timeout.
func waitFor(ctx context.Context, t *testing.T, what string, timeout time.Duration, check func(context.Context) (bool, string, error)) error {
	t.Helper()

	lastState := ""
	done, err := pollUntil(ctx, timeout, mk8sPollInterval, func() (bool, error) {
		c, cancel := context.WithTimeout(ctx, mk8sCallTimeout)
		defer cancel()
		ok, state, err := check(c)
		if err == nil && state != lastState {
			t.Logf("%s: %s", what, state)
			lastState = state
		}
		return ok, err
	})
	if err != nil {
		return fmt.Errorf("waiting for %s (last state %q): %w", what, lastState, err)
	}
	if !done {
		return fmt.Errorf("timed out after %s waiting for %s (last state %q)", timeout, what, lastState)
	}
	return nil
}

// cleanupBudget hands out cleanup contexts from one absolute end time, so a
// slow cleanup cannot starve the cleanups after it.
type cleanupBudget struct {
	end time.Time
}

// context returns a context for one cleanup step: at most own, and never
// more than the time left after reserving mk8sCleanupStepMinimum for each of
// the laterSteps cleanups that run after it. The final cleanups (subnet,
// network and SSH key) count as one later step.
func (b cleanupBudget) context(own time.Duration, laterSteps int) (context.Context, context.CancelFunc) {
	d := min(own, time.Until(b.end)-time.Duration(laterSteps)*mk8sCleanupStepMinimum)
	return context.WithTimeout(context.Background(), max(d, 0))
}

// finalContext returns a context for one of the last cleanups (subnet,
// network and SSH key): whatever time is left, and at least
// mk8sCleanupStepMinimum.
func (b cleanupBudget) finalContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), max(time.Until(b.end), mk8sCleanupStepMinimum))
}

// call runs one API call with its own mk8sCallTimeout.
func call[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	c, cancel := context.WithTimeout(ctx, mk8sCallTimeout)
	defer cancel()
	return fn(c)
}

// callErr is call for API calls that return only an error.
func callErr(ctx context.Context, fn func(context.Context) error) error {
	c, cancel := context.WithTimeout(ctx, mk8sCallTimeout)
	defer cancel()
	return fn(c)
}

func updateClusterVersion(ctx context.Context, client *verda.Client, location, clusterName, version string) (*verda.ManagedKubernetesCluster, error) {
	return call(ctx, func(c context.Context) (*verda.ManagedKubernetesCluster, error) {
		return client.ManagedKubernetes.Update(c, location, clusterName, verda.UpdateManagedKubernetesClusterRequest{KubernetesVersion: version})
	})
}

// requireAPIError fails the test unless err is an *verda.APIError with the
// given status, code and details.reason.
func requireAPIError(t *testing.T, err error, status int, code, reason string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected API error %d %s/%s, got nil", status, code, reason)
	}
	var apiErr *verda.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *verda.APIError %d %s/%s, got %T: %v", status, code, reason, err, err)
	}
	if got := apiErrorReason(apiErr); apiErr.StatusCode != status || apiErr.Code != code || got != reason {
		t.Fatalf("expected API error %d %s/%s, got %d %s/%s: %s", status, code, reason, apiErr.StatusCode, apiErr.Code, got, apiErr.Message)
	}
}

// isAPIError reports whether err is an *verda.APIError with status and one
// of reasons.
func isAPIError(err error, status int, reasons ...string) bool {
	var apiErr *verda.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
		return false
	}
	return slices.Contains(reasons, apiErrorReason(apiErr))
}

// isNotFound reports whether err is an *verda.APIError with status 404,
// whatever its reason.
func isNotFound(err error) bool {
	var apiErr *verda.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// isTransient reports whether err is a timed-out call, a transport error (no
// HTTP response, such as a connection reset), a 429 or a 5xx response.
func isTransient(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var apiErr *verda.APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= http.StatusInternalServerError)
}

func apiErrorCode(err error) string {
	var apiErr *verda.APIError
	if !errors.As(err, &apiErr) {
		return ""
	}
	return apiErr.Code
}

// apiErrorReason decodes details.reason; APIError.Details holds the details
// object as compact JSON text.
func apiErrorReason(apiErr *verda.APIError) string {
	var details struct {
		Reason string `json:"reason"`
	}
	if apiErr.Details == "" || json.Unmarshal([]byte(apiErr.Details), &details) != nil {
		return ""
	}
	return details.Reason
}

// assertClusterSpec compares the stable fields of a cluster with the create
// request; phase and conditions change over time and are not compared.
func assertClusterSpec(t *testing.T, source string, got *verda.ManagedKubernetesCluster, req verda.CreateManagedKubernetesClusterRequest, version string) {
	t.Helper()

	checks := []struct{ field, got, want string }{
		{"name", got.Name, req.Name},
		{"kubernetes_version", got.KubernetesVersion, version},
		{"location", got.Location, req.Location},
		{"network", got.NetworkID, req.NetworkID},
		{"subnet", got.SubnetID, req.SubnetID},
		{"pod_cidr", got.PodCIDR, req.PodCIDR},
		{"service_cidr", got.ServiceCIDR, req.ServiceCIDR},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: cluster %s = %q, want %q", source, c.field, c.got, c.want)
		}
	}
}

// assertNodePoolSpec compares the stable fields of a node pool with the
// create request.
func assertNodePoolSpec(t *testing.T, source string, got *verda.ManagedKubernetesNodePool, req verda.CreateManagedKubernetesNodePoolRequest, replicas int) {
	t.Helper()

	if got.Name != req.Name {
		t.Errorf("%s: pool name = %q, want %q", source, got.Name, req.Name)
	}
	if got.InstanceType != req.InstanceType {
		t.Errorf("%s: pool instance_type = %q, want %q", source, got.InstanceType, req.InstanceType)
	}
	if got.Replicas != replicas {
		t.Errorf("%s: pool replicas = %d, want %d", source, got.Replicas, replicas)
	}
	if !slices.Equal(got.SSHKeyIDs, req.SSHKeyIDs) {
		t.Errorf("%s: pool ssh_key_ids = %v, want %v", source, got.SSHKeyIDs, req.SSHKeyIDs)
	}
}

func describeConditions(conditions []verda.ManagedKubernetesCondition) string {
	if len(conditions) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(conditions))
	for _, c := range conditions {
		parts = append(parts, fmt.Sprintf("%s=%s (%s)", c.Type, c.Status, c.Reason))
	}
	return strings.Join(parts, ", ")
}

// missingKubeconfigID returns an id in the server's "<cluster>-kc-<suffix>"
// form that does not exist.
func missingKubeconfigID(t *testing.T, clusterName string) string {
	t.Helper()
	return clusterName + "-kc-" + randomHex(t)
}

// randomHex returns 8 random hex characters.
func randomHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("read random bytes: %v", err)
	}
	return hex.EncodeToString(b)
}
