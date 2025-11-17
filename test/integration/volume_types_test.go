//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
)

func TestVolumeTypesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)

	t.Run("get all volume types", func(t *testing.T) {
		ctx := context.Background()
		volumeTypes, err := client.VolumeTypes.GetAllVolumeTypes(ctx)
		if err != nil {
			t.Errorf("failed to get volume types: %v", err)
		}
		t.Logf("Found %d volume types", len(volumeTypes))

		// Verify structure if volume types exist
		if len(volumeTypes) > 0 {
			for i, vt := range volumeTypes {
				if i < 5 { // Log first 5
					t.Logf("Volume Type: %s - Price: %f %s/GB/month, Shared: %v, IOPS: %s",
						vt.Type, vt.Price.MonthlyPerGB, vt.Price.Currency, vt.IsSharedFS, vt.IOPS)
				}
				if vt.Type == "" {
					t.Errorf("volume type %d missing Type", i)
				}
				if vt.Price.MonthlyPerGB <= 0 {
					t.Errorf("volume type %d has invalid price: %f", i, vt.Price.MonthlyPerGB)
				}
				if vt.Price.Currency == "" {
					t.Errorf("volume type %d missing Currency", i)
				}
				if vt.IOPS == "" {
					t.Errorf("volume type %d missing IOPS", i)
				}
			}
		}
	})

	t.Run("test deprecated Get method", func(t *testing.T) {
		ctx := context.Background()
		volumeTypes, err := client.VolumeTypes.Get(ctx)
		if err != nil {
			t.Errorf("failed to get volume types with deprecated method: %v", err)
		}
		t.Logf("Deprecated Get method returned %d volume types", len(volumeTypes))
	})

	t.Run("verify volume type constants", func(t *testing.T) {
		ctx := context.Background()
		volumeTypes, err := client.VolumeTypes.GetAllVolumeTypes(ctx)
		if err != nil {
			t.Errorf("failed to get volume types: %v", err)
		}

		// Check that returned types match expected constants
		expectedTypes := map[string]bool{
			"HDD":                 false,
			"NVMe":                false,
			"HDD_Shared":          false,
			"NVMe_Shared":         false,
			"NVMe_Local_Storage":  false,
			"NVMe_Shared_Cluster": false,
			"NVMe_OS_Cluster":     false,
		}

		for _, vt := range volumeTypes {
			if _, exists := expectedTypes[vt.Type]; exists {
				expectedTypes[vt.Type] = true
			}
		}

		foundCount := 0
		for typeName, found := range expectedTypes {
			if found {
				foundCount++
				t.Logf("Found expected volume type: %s", typeName)
			}
		}

		if foundCount == 0 {
			t.Error("Expected to find at least one known volume type")
		}
	})

	t.Run("verify performance characteristics", func(t *testing.T) {
		ctx := context.Background()
		volumeTypes, err := client.VolumeTypes.GetAllVolumeTypes(ctx)
		if err != nil {
			t.Errorf("failed to get volume types: %v", err)
		}

		for _, vt := range volumeTypes {
			// Verify performance metrics are present and reasonable
			if vt.BurstBandwidth < 0 {
				t.Errorf("volume type %s has negative BurstBandwidth: %f", vt.Type, vt.BurstBandwidth)
			}
			if vt.ContinuousBandwidth < 0 {
				t.Errorf("volume type %s has negative ContinuousBandwidth: %f", vt.Type, vt.ContinuousBandwidth)
			}
			if vt.InternalNetworkSpeed < 0 {
				t.Errorf("volume type %s has negative InternalNetworkSpeed: %f", vt.Type, vt.InternalNetworkSpeed)
			}

			// Continuous should not exceed burst
			if vt.ContinuousBandwidth > vt.BurstBandwidth {
				t.Errorf("volume type %s continuous bandwidth (%f) exceeds burst bandwidth (%f)",
					vt.Type, vt.ContinuousBandwidth, vt.BurstBandwidth)
			}
		}
	})

	t.Run("verify pricing information", func(t *testing.T) {
		ctx := context.Background()
		volumeTypes, err := client.VolumeTypes.GetAllVolumeTypes(ctx)
		if err != nil {
			t.Errorf("failed to get volume types: %v", err)
		}

		for _, vt := range volumeTypes {
			// Verify pricing is present and reasonable
			if vt.Price.MonthlyPerGB <= 0 {
				t.Errorf("volume type %s has invalid price: %f", vt.Type, vt.Price.MonthlyPerGB)
			}
			if vt.Price.Currency == "" {
				t.Errorf("volume type %s missing currency", vt.Type)
			}

			// Log pricing for manual verification
			t.Logf("Volume type %s: %f %s/GB/month", vt.Type, vt.Price.MonthlyPerGB, vt.Price.Currency)
		}
	})
}
