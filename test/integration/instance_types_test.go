//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestInstanceTypesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get instance types", func(t *testing.T) {
		instanceTypes, err := client.InstanceTypes.Get(ctx, "usd")
		if err != nil {
			t.Errorf("failed to get instance types: %v", err)
		}
		t.Logf("Found %d instance types", len(instanceTypes))

		// Verify instance types have expected fields
		if len(instanceTypes) > 0 {
			it := instanceTypes[0]
			t.Logf("Sample instance type: %s (%s) - $%.2f/hr", it.InstanceType, it.Name, it.PricePerHour.Float64())

			if it.ID == "" {
				t.Error("instance type missing ID")
			}
			if it.InstanceType == "" {
				t.Error("instance type missing InstanceType")
			}
			if it.Model == "" {
				t.Error("instance type missing Model")
			}

			// Test getting specific instance type
			// Note: This endpoint may not be available in staging or for all instance types
			specificType, err := client.InstanceTypes.GetByInstanceType(ctx, it.InstanceType, false, "", "usd")
			if err != nil {
				// Handle 404 gracefully - endpoint may not be available
				if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
					t.Logf("⚠️  GetByInstanceType endpoint not available for %s (404) - this may be expected", it.InstanceType)
				} else {
					t.Errorf("failed to get specific instance type: %v", err)
				}
			} else if specificType != nil {
				t.Logf("Retrieved specific instance type: %s - $%.2f/hr", specificType.InstanceType, specificType.PricePerHour.Float64())

				if specificType.InstanceType != it.InstanceType {
					t.Errorf("expected instance type %s, got %s", it.InstanceType, specificType.InstanceType)
				}
			}
		}
	})

	t.Run("get price history", func(t *testing.T) {
		priceHistory, err := client.InstanceTypes.GetPriceHistory(ctx, 1, "usd")
		if err != nil {
			t.Errorf("failed to get price history: %v", err)
		}
		t.Logf("Found price history for %d instance types", len(priceHistory))

		// Verify price history structure
		if len(priceHistory) > 0 {
			for instanceType, records := range priceHistory {
				t.Logf("Instance type %s has %d price records", instanceType, len(records))
				if len(records) > 0 {
					// Check first record
					r := records[0]
					if r.Date == "" {
						t.Errorf("price record missing date for %s", instanceType)
					}
					if r.Currency == "" {
						t.Errorf("price record missing currency for %s", instanceType)
					}
				}
				break // Just check first instance type
			}
		}
	})
}
