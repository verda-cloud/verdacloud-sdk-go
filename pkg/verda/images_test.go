package verda

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestImagesService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get instance images", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Images.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(images) == 0 {
			t.Error("expected at least one image")
		}

		// Verify first image has expected fields
		if len(images) > 0 {
			image := images[0]
			if image.ID == "" {
				t.Error("expected image to have an ID")
			}
			if image.ImageType == "" {
				t.Error("expected image to have an ImageType")
			}
			if image.Name == "" {
				t.Error("expected image to have a Name")
			}
			if image.Category == "" {
				t.Error("expected image to have a Category")
			}
			// Details can be empty, so just check it's not nil
			if image.Details == nil {
				t.Error("expected image to have a Details field (can be empty slice)")
			}
		}
	})

	t.Run("verify image fields structure", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Images.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(images) > 0 {
			for i, image := range images {
				// Check that each image has basic required fields
				if image.ID == "" {
					t.Errorf("image %d missing ID", i)
				}
				if image.ImageType == "" {
					t.Errorf("image %d missing ImageType", i)
				}
				if image.Name == "" {
					t.Errorf("image %d missing Name", i)
				}

				// IsDefault and IsCluster are booleans, they always have a value (true/false)
				// No need to check them for nil

				// Details should be a slice (can be empty)
				if image.Details == nil {
					t.Errorf("image %d has nil Details field", i)
				}
			}
		}
	})

	t.Run("verify at least one default image exists", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Images.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		hasDefaultImage := false
		for _, image := range images {
			if image.IsDefault {
				hasDefaultImage = true
				break
			}
		}

		if !hasDefaultImage {
			t.Error("expected at least one default image")
		}
	})

	t.Run("verify images have proper categories", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Images.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(images) > 0 {
			validCategories := map[string]bool{
				"gpu": true,
				"ml":  true,
				"cpu": true,
				"":    false, // empty category should not exist
			}

			for _, image := range images {
				if image.Category == "" {
					t.Errorf("image %s has empty category", image.ID)
				}
				// We don't enforce specific categories, just that they exist
			}

			_ = validCategories // Used for documentation purposes
		}
	})
}

func TestImagesService_GetClusterImages(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get cluster images", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Images.GetClusterImages(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(images) == 0 {
			t.Error("expected at least one cluster image")
		}

		// Verify first image has expected fields
		if len(images) > 0 {
			image := images[0]
			if image.Name == "" {
				t.Error("expected cluster image to have a Name")
			}
			if image.ImageType == "" {
				t.Error("expected cluster image to have an ImageType")
			}
			if image.ID == "" {
				t.Error("expected cluster image to have an ID")
			}
		}
	})

	t.Run("verify cluster image structure", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Images.GetClusterImages(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(images) > 0 {
			for i, image := range images {
				if image.ID == "" {
					t.Errorf("cluster image %d missing ID", i)
				}
				if image.ImageType == "" {
					t.Errorf("cluster image %d missing ImageType", i)
				}
				if image.Name == "" {
					t.Errorf("cluster image %d missing Name", i)
				}
				if image.Category == "" {
					t.Errorf("cluster image %d missing Category", i)
				}
				if len(image.Details) == 0 {
					t.Errorf("cluster image %d has no Details", i)
				}
				// IsDefault and IsCluster are booleans, always have values
			}
		}
	})

	t.Run("verify cluster images are marked as cluster images", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Images.GetClusterImages(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		hasClusterImage := false
		for _, image := range images {
			if image.IsCluster {
				hasClusterImage = true
				break
			}
		}

		if !hasClusterImage {
			t.Error("expected at least one image with IsCluster=true")
		}
	})

	t.Run("verify cluster images have details", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Images.GetClusterImages(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(images) > 0 {
			for _, image := range images {
				if len(image.Details) == 0 {
					t.Errorf("cluster image %s has no details", image.Name)
				}
			}
		}
	})
}
