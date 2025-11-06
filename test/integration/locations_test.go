//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
)

func TestLocations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := createTestClient(t)

	t.Run("get_locations", func(t *testing.T) {
		ctx := context.Background()
		locations, err := client.Locations.Get(ctx)
		if err != nil {
			t.Errorf("failed to get locations: %v", err)
		}
		if len(locations) == 0 {
			t.Error("expected at least one location")
		}
		t.Logf("Found %d locations", len(locations))
	})
}
