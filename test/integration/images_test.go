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

	t.Run("get images by instance type", func(t *testing.T) {
		images, err := client.Images.GetImagesByInstanceType(ctx, "8B300.240V")
		if err != nil {
			t.Errorf("failed to get images by instance type: %v", err)
		}
		t.Logf("Found %d images for instance type 8B300.240V", len(images))

		if len(images) > 0 {
			for i, img := range images {
				if i < 3 {
					t.Logf("Image: %s (%s) - Default: %v, Category: %s",
						img.Name, img.ImageType, img.IsDefault, img.Category)
				}
			}
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
				t.Logf("Cluster Image: %s (%s) - Default: %v, Category: %s, Details: %v",
					img.Name, img.ImageType, img.IsDefault, img.Category, img.Details)
			}
		}

		// Verify we got data
		if len(clusterImages) == 0 {
			t.Error("expected at least one cluster image")
		}
	})
}
