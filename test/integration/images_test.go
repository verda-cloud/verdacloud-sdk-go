//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
)

func TestImagesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get instance images", func(t *testing.T) {
		images, err := client.Images.Get(ctx)
		if err != nil {
			t.Errorf("failed to get images: %v", err)
		}
		t.Logf("Found %d instance images", len(images))

		// Log some image details
		if len(images) > 0 {
			for i, img := range images {
				if i < 3 { // Only log first 3
					t.Logf("Image: %s (%s) - Default: %v, Category: %s",
						img.Name, img.ImageType, img.IsDefault, img.Category)
				}
			}
		}

		// Verify we got data
		if len(images) == 0 {
			t.Error("expected at least one instance image")
		}
	})

	t.Run("get cluster images", func(t *testing.T) {
		clusterImages, err := client.Images.GetClusterImages(ctx)
		if err != nil {
			t.Errorf("failed to get cluster images: %v", err)
		}
		t.Logf("Found %d cluster images", len(clusterImages))

		// Log cluster image details
		if len(clusterImages) > 0 {
			for _, img := range clusterImages {
				t.Logf("Cluster Image: %s (v%s) - Available: %v, Description: %s",
					img.Name, img.Version, img.Available, img.Description)
			}
		}

		// Verify we got data
		if len(clusterImages) == 0 {
			t.Error("expected at least one cluster image")
		}
	})
}
