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

package integration

import (
	"regexp"
	"testing"
	"time"
)

// This file has no build tag, so `make test` checks without credentials
// which resources TestSweepManagedKubernetesLeftovers may delete.

// mk8sSweepMinAge is longer than any lifecycle run (at most about 21
// minutes: the 9-minute body plus every cleanup budget), so the sweeper does
// not delete the resources of a run that is still in progress.
const mk8sSweepMinAge = 30 * time.Minute

// Names created by TestManagedKubernetesLifecycle: the cluster and the SSH
// key are "sdk-e2e-<8 hex>", the network is "sdk-e2e-net-<8 hex>".
var (
	mk8sSweepClusterName = regexp.MustCompile(`^sdk-e2e-[0-9a-f]{8}$`)
	mk8sSweepNetworkName = regexp.MustCompile(`^sdk-e2e-net-[0-9a-f]{8}$`)
	mk8sSweepSSHKeyName  = regexp.MustCompile(`^sdk-e2e-[0-9a-f]{8}$`)
)

// mk8sSweepAction is what the sweeper does with one resource.
type mk8sSweepAction int

const (
	// mk8sSweepIgnore: the lifecycle test does not create this name.
	mk8sSweepIgnore mk8sSweepAction = iota
	// mk8sSweepKeep: a test resource younger than mk8sSweepMinAge. It may
	// belong to a running test.
	mk8sSweepKeep
	// mk8sSweepReport: a test resource without created_at. Its age is
	// unknown, so it is reported and not deleted.
	mk8sSweepReport
	// mk8sSweepDelete: a test resource at least mk8sSweepMinAge old.
	mk8sSweepDelete
)

// mk8sSweepDecide decides what the sweeper does with the resource called
// name and created at createdAt, when the sweep runs at now. Only names that
// match pattern are test resources.
func mk8sSweepDecide(pattern *regexp.Regexp, name string, createdAt, now time.Time) mk8sSweepAction {
	switch {
	case !pattern.MatchString(name):
		return mk8sSweepIgnore
	case createdAt.IsZero():
		return mk8sSweepReport
	case createdAt.After(now.Add(-mk8sSweepMinAge)):
		return mk8sSweepKeep
	default:
		return mk8sSweepDelete
	}
}

func TestMk8sSweepDecide(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	old := now.Add(-mk8sSweepMinAge)
	tests := []struct {
		name      string
		pattern   *regexp.Regexp
		resource  string
		createdAt time.Time
		want      mk8sSweepAction
	}{
		{"cluster at the age limit", mk8sSweepClusterName, "sdk-e2e-0123abcd", old, mk8sSweepDelete},
		{"cluster one second younger", mk8sSweepClusterName, "sdk-e2e-0123abcd", old.Add(time.Second), mk8sSweepKeep},
		{"cluster created in the future", mk8sSweepClusterName, "sdk-e2e-0123abcd", now.Add(time.Minute), mk8sSweepKeep},
		{"cluster without created_at", mk8sSweepClusterName, "sdk-e2e-0123abcd", time.Time{}, mk8sSweepReport},
		{"network name as a cluster", mk8sSweepClusterName, "sdk-e2e-net-0123abcd", old, mk8sSweepIgnore},
		{"subnet name as a cluster", mk8sSweepClusterName, "sdk-e2e-subnet-0123abcd", old, mk8sSweepIgnore},
		{"uppercase hex", mk8sSweepClusterName, "sdk-e2e-0123ABCD", old, mk8sSweepIgnore},
		{"seven hex characters", mk8sSweepClusterName, "sdk-e2e-0123abc", old, mk8sSweepIgnore},
		{"nine hex characters", mk8sSweepClusterName, "sdk-e2e-0123abcde", old, mk8sSweepIgnore},
		{"text before the name", mk8sSweepClusterName, "my-sdk-e2e-0123abcd", old, mk8sSweepIgnore},
		{"not hex", mk8sSweepClusterName, "sdk-e2e-keepme12", old, mk8sSweepIgnore},
		{"network at the age limit", mk8sSweepNetworkName, "sdk-e2e-net-0123abcd", old, mk8sSweepDelete},
		{"network one second younger", mk8sSweepNetworkName, "sdk-e2e-net-0123abcd", old.Add(time.Second), mk8sSweepKeep},
		{"cluster name as a network", mk8sSweepNetworkName, "sdk-e2e-0123abcd", old, mk8sSweepIgnore},
		{"SSH key at the age limit", mk8sSweepSSHKeyName, "sdk-e2e-0123abcd", old, mk8sSweepDelete},
		{"SSH key without created_at", mk8sSweepSSHKeyName, "sdk-e2e-0123abcd", time.Time{}, mk8sSweepReport},
		{"pool name as an SSH key", mk8sSweepSSHKeyName, "pool-0123abcd", old, mk8sSweepIgnore},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mk8sSweepDecide(tt.pattern, tt.resource, tt.createdAt, now); got != tt.want {
				t.Errorf("mk8sSweepDecide(%s, %q, %s) = %d, want %d", tt.pattern, tt.resource, tt.createdAt, got, tt.want)
			}
		})
	}
}
