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

func TestLongTermIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get instance long-term periods", func(t *testing.T) {
		instancePeriods, err := client.LongTerm.GetInstancePeriods(ctx)
		if err != nil {
			t.Errorf("failed to get instance periods: %v", err)
		}
		t.Logf("Found %d instance long-term periods", len(instancePeriods))

		// Log instance period details
		if len(instancePeriods) > 0 {
			for _, period := range instancePeriods {
				t.Logf("Instance Period: %s - %s (%d %s, %.2f%% discount, enabled: %v)",
					period.Code, period.Name, period.UnitValue, period.UnitName,
					period.DiscountPercentage, period.IsEnabled)
			}
		}

		// Verify we got data
		if len(instancePeriods) == 0 {
			t.Error("expected at least one instance period")
		}
	})

	t.Run("get general long-term periods", func(t *testing.T) {
		generalPeriods, err := client.LongTerm.GetPeriods(ctx)
		if err != nil {
			t.Errorf("failed to get general periods: %v", err)
		}
		t.Logf("Found %d general long-term periods", len(generalPeriods))

		// Log general period details
		if len(generalPeriods) > 0 {
			for _, period := range generalPeriods {
				t.Logf("General Period: %s - %s (%d %s, %.2f%% discount, enabled: %v)",
					period.Code, period.Name, period.UnitValue, period.UnitName,
					period.DiscountPercentage, period.IsEnabled)
			}
		}

		// Verify we got data
		if len(generalPeriods) == 0 {
			t.Error("expected at least one general period")
		}
	})

	t.Run("get cluster long-term periods", func(t *testing.T) {
		clusterPeriods, err := client.LongTerm.GetClusterPeriods(ctx)
		if err != nil {
			t.Errorf("failed to get cluster periods: %v", err)
		}
		t.Logf("Found %d cluster long-term periods", len(clusterPeriods))

		// Log cluster period details
		if len(clusterPeriods) > 0 {
			for _, period := range clusterPeriods {
				t.Logf("Cluster Period: %s - %s (%d %s, %.2f%% discount, enabled: %v)",
					period.Code, period.Name, period.UnitValue, period.UnitName,
					period.DiscountPercentage, period.IsEnabled)
			}
		}

		// Note: Cluster periods may not be available in staging environment
		if len(clusterPeriods) == 0 {
			t.Log("⚠️  No cluster periods found - this may be expected in staging environment")
		}
	})
}
