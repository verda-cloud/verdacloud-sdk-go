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
