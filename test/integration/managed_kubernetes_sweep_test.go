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
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// mk8sSweepMargin is left before the test deadline for reporting.
const mk8sSweepMargin = 30 * time.Second

// requireMk8sOptIn skips the test unless VERDA_E2E_MK8S=1. An opted-in test
// fails without credentials, because getTestClient would skip it and the run
// would pass without testing anything.
func requireMk8sOptIn(t *testing.T) {
	t.Helper()
	if os.Getenv(mk8sOptInEnv) != "1" {
		t.Skipf("creates or deletes billable managed Kubernetes resources; set %s=1 or run make test-integration-mk8s", mk8sOptInEnv)
	}
	if os.Getenv("VERDA_CLIENT_ID") == "" || os.Getenv("VERDA_CLIENT_SECRET") == "" {
		t.Fatalf("%s=1 needs VERDA_CLIENT_ID and VERDA_CLIENT_SECRET", mk8sOptInEnv)
	}
}

// TestSweepManagedKubernetesLeftovers deletes the leftovers of killed
// lifecycle runs. A killed run (Ctrl-C, a cancelled CI job, a
// `go test -timeout` panic in another test) does not run its cleanups, and
// leaves its cluster, node pool, network, subnet and SSH key behind. The
// sweeper deletes resources with the lifecycle test's names that are at
// least mk8sSweepMinAge old (see mk8sSweepDecide), and never instances.
// `make test-integration-mk8s` runs it first.
//
// It runs only with VERDA_E2E_MK8S=1. Its name does not start with
// TestManagedKubernetes, so `-run TestManagedKubernetes` does not select it.
func TestSweepManagedKubernetesLeftovers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}
	requireMk8sOptIn(t)

	client := getTestClient(t)
	deadline, ok := t.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Minute)
	}
	ctx, cancel := context.WithDeadline(t.Context(), deadline.Add(-mk8sSweepMargin))
	defer cancel()

	now := time.Now()
	sweepable := func(what string, pattern *regexp.Regexp, name string, createdAt time.Time) bool {
		switch mk8sSweepDecide(pattern, name, createdAt, now) {
		case mk8sSweepDelete:
			return true
		case mk8sSweepReport:
			t.Errorf("sweep: %s has no created_at; not deleting it, because it may belong to a running test", what)
		case mk8sSweepKeep:
			t.Logf("sweep: keeping %s, created %s ago; it may belong to a running test", what, now.Sub(createdAt).Round(time.Second))
		}
		return false
	}

	// Clusters first: their pools hold the workers in the subnet and the SSH
	// key.
	clusters, err := call(ctx, client.ManagedKubernetes.List)
	if err != nil {
		t.Fatalf("List clusters: %v", err)
	}
	for _, c := range clusters {
		if sweepable("cluster "+c.Location+"/"+c.Name, mk8sSweepClusterName, c.Name, c.CreatedAt) {
			sweepCluster(ctx, t, client, c.Location, c.Name)
		}
	}

	networks, err := call(ctx, client.PrivateNetworks.Get)
	if err != nil {
		t.Fatalf("list private networks: %v", err)
	}
	for _, n := range networks {
		if sweepable("private network "+n.Name, mk8sSweepNetworkName, n.Name, n.CreatedAt) {
			sweepNetwork(ctx, t, client, n)
		}
	}

	keys, err := call(ctx, client.SSHKeys.GetAllSSHKeys)
	if err != nil {
		t.Fatalf("list SSH keys: %v", err)
	}
	for _, k := range keys {
		if !sweepable("SSH key "+k.Name, mk8sSweepSSHKeyName, k.Name, k.CreatedAt) {
			continue
		}
		t.Logf("sweep: deleting SSH key %s (%s)", k.ID, k.Name)
		if err := callErr(ctx, func(c context.Context) error { return client.SSHKeys.DeleteSSHKey(c, k.ID) }); err != nil {
			t.Errorf("sweep: SSH key %s (%s): %v", k.ID, k.Name, err)
		}
	}
}

// sweepCluster deletes the cluster's kubeconfigs and node pools, then the
// cluster, and waits for each to be gone.
func sweepCluster(ctx context.Context, t *testing.T, client *verda.Client, location, name string) {
	t.Helper()

	t.Logf("sweep: deleting cluster %s/%s", location, name)
	cleanupKubeconfigs(ctx, t, client, location, name, "")
	pools, err := call(ctx, func(c context.Context) ([]verda.ManagedKubernetesNodePool, error) {
		return client.ManagedKubernetes.ListNodePools(c, location, name)
	})
	if isAPIError(err, http.StatusNotFound, mk8sReasonClusterNotFound) {
		return
	}
	if err != nil {
		t.Errorf("sweep: list node pools of %s/%s: %v", location, name, err)
		return
	}
	for _, p := range pools {
		if err := deleteNodePoolAndWait(ctx, t, client, location, name, p.Name, true); err != nil {
			t.Errorf("sweep: node pool %s/%s/%s: %v", location, name, p.Name, err)
			return
		}
	}
	if err := deleteClusterAndWait(ctx, t, client, location, name, true); err != nil {
		t.Errorf("sweep: cluster %s/%s: %v", location, name, err)
	}
}

// sweepNetwork deletes the network's subnets, then the network. A subnet is
// deleted only after no instance remains in it; the sweeper never deletes
// instances itself, so a worker that outlived its pool is reported instead.
func sweepNetwork(ctx context.Context, t *testing.T, client *verda.Client, network verda.PrivateNetwork) {
	t.Helper()

	t.Logf("sweep: deleting private network %s (%s)", network.ID, network.Name)
	subnets, err := call(ctx, func(c context.Context) ([]verda.Subnet, error) {
		return client.PrivateNetworks.GetSubnets(c, network.ID)
	})
	if err != nil {
		t.Errorf("sweep: list subnets of network %s: %v", network.ID, err)
		return
	}
	for _, s := range subnets {
		var leftover []string
		err := waitFor(ctx, t, "no instances in subnet "+s.ID, mk8sCleanupWorkers, func(c context.Context) (bool, string, error) {
			instances, err := client.Instances.List(c, nil)
			if err != nil {
				if isTransient(err) {
					return false, "list failed", nil
				}
				return false, "", err
			}
			leftover = filterInstancesInSubnet(instances, s.ID)
			return len(leftover) == 0, fmt.Sprintf("%d instances", len(leftover)), nil
		})
		if err != nil {
			t.Errorf("sweep: subnet %s of network %s: %v; instances: %v", s.ID, network.ID, err, leftover)
			return
		}
		if err := callErr(ctx, func(c context.Context) error { return client.PrivateNetworks.DeleteSubnet(c, network.ID, s.ID) }); err != nil {
			t.Errorf("sweep: subnet %s of network %s: %v", s.ID, network.ID, err)
			return
		}
	}
	if err := callErr(ctx, func(c context.Context) error { return client.PrivateNetworks.Delete(c, network.ID) }); err != nil {
		t.Errorf("sweep: private network %s (%s): %v", network.ID, network.Name, err)
	}
}
