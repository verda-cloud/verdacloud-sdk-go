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

		// Verify we got data
		if len(clusterPeriods) == 0 {
			t.Error("expected at least one cluster period")
		}
	})
}
