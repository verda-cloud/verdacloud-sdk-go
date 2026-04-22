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
// +build integration

package integration

import (
	"context"
	"testing"
)

func TestInstanceAvailabilityIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get all availabilities", func(t *testing.T) {
		availabilities, err := client.InstanceAvailability.GetAllAvailabilities(ctx, false, "")
		if err != nil {
			t.Errorf("failed to get instance availabilities: %v", err)
		}
		t.Logf("Found %d location availability entries", len(availabilities))

		// Log some availability info
		if len(availabilities) > 0 {
			for _, avail := range availabilities {
				t.Logf("Location %s has %d available instance types", avail.LocationCode, len(avail.Availabilities))
			}
		}

		// Test checking specific instance type availability
		if len(availabilities) > 0 && len(availabilities[0].Availabilities) > 0 {
			instanceType := availabilities[0].Availabilities[0]
			available, err := client.InstanceAvailability.GetInstanceTypeAvailability(ctx, instanceType, false, "")
			if err != nil {
				t.Errorf("failed to check instance availability: %v", err)
			}
			t.Logf("Instance type %s available: %v", instanceType, available)
		}
	})
}
