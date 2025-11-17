//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestClustersIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("list clusters", func(t *testing.T) {
		clusters, err := client.Clusters.Get(ctx)
		if err != nil {
			// Check if it's a 404 error (not supported on staging)
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Clusters endpoint not available (404)")
				return
			}
			t.Errorf("failed to get clusters: %v", err)
		}
		t.Logf("Found %d clusters", len(clusters))
	})

	t.Run("get cluster types", func(t *testing.T) {
		clusterTypes, err := client.Clusters.GetClusterTypes(ctx, "usd")
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Cluster types endpoint not available (404)")
				return
			}
			t.Errorf("failed to get cluster types: %v", err)
		}
		t.Logf("Found %d cluster types", len(clusterTypes))

		// Test checking specific cluster type availability
		if len(clusterTypes) > 0 {
			clusterType := clusterTypes[0].ClusterType
			available, err := client.Clusters.CheckClusterTypeAvailability(ctx, clusterType, "")
			if err != nil {
				t.Errorf("failed to check cluster type availability: %v", err)
			}
			t.Logf("Cluster type %s available: %v", clusterType, available)
		}
	})

	t.Run("get cluster availabilities", func(t *testing.T) {
		availabilities, err := client.Clusters.GetAvailabilities(ctx, "")
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Cluster availabilities endpoint not available (404)")
				return
			}
			t.Errorf("failed to get cluster availabilities: %v", err)
		}
		t.Logf("Found %d cluster availability entries", len(availabilities))
	})

	t.Run("get cluster images", func(t *testing.T) {
		images, err := client.Clusters.GetImages(ctx)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Cluster images endpoint not available (404)")
				return
			}
			t.Errorf("failed to get cluster images: %v", err)
		}
		t.Logf("Found %d cluster images", len(images))
	})
}
