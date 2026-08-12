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
	"net/http"
	"strings"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// TestTagsLifecycleIntegration exercises the tag endpoints against a real volume,
// which is the cheapest taggable resource to provision.
func TestTagsLifecycleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	volumeName := generateRandomName("integration-test-tags")

	volumeID, err := client.Volumes.CreateVolume(ctx, verda.VolumeCreateRequest{
		Type:         verda.VolumeTypeNVMe,
		LocationCode: verda.LocationFIN03,
		Size:         50,
		Name:         volumeName,
		Tags:         []verda.TagRequest{{Key: "created-by", Value: "sdk-integration-test"}},
	})
	if err != nil {
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == http.StatusBadRequest &&
			(strings.Contains(apiErr.Message, "Volume limit exceeded") || strings.Contains(apiErr.Message, "Storage limit exceeded")) {
			t.Skipf("Skipping tag lifecycle due to quota: %v", apiErr)
		}
		t.Fatalf("failed to create volume: %v", err)
	}

	t.Logf("✅ Created volume %s (ID: %s)", volumeName, volumeID)

	// Always clean the volume up, even if a subtest fails
	t.Cleanup(func() {
		if err := client.Volumes.DeleteVolume(context.Background(), volumeID, true); err != nil {
			t.Logf("⚠️  Warning: failed to delete volume %s: %v", volumeID, err)
			return
		}
		t.Logf("🧹 Deleted volume %s", volumeID)
	})

	t.Run("tags supplied at creation are persisted", func(t *testing.T) {
		volume, err := client.Volumes.GetVolume(ctx, volumeID)
		if err != nil {
			t.Fatalf("failed to get volume: %v", err)
		}

		if !hasTag(volume.Tags, "created-by", "sdk-integration-test") {
			t.Errorf("expected the creation tag on the volume, got %+v", volume.Tags)
		}
	})

	t.Run("add a key-value tag", func(t *testing.T) {
		tag, err := client.Volumes.AddTag(ctx, volumeID, verda.TagRequest{Key: "environment", Value: "test"})
		if err != nil {
			t.Fatalf("failed to add tag: %v", err)
		}

		if tag.ID == "" {
			t.Error("expected the created tag to have an ID")
		}
		if tag.Key != "environment" {
			t.Errorf("expected key environment, got %s", tag.Key)
		}
		if tag.Value != "test" {
			t.Errorf("expected value test, got %s", tag.Value)
		}

		t.Logf("✅ Added tag %s=%s (ID: %s)", tag.Key, tag.Value, tag.ID)
	})

	t.Run("add a freeform tag", func(t *testing.T) {
		tag, err := client.Volumes.AddTag(ctx, volumeID, verda.TagRequest{Key: "benchmark"})
		if err != nil {
			t.Fatalf("failed to add freeform tag: %v", err)
		}

		if tag.Value != "" {
			t.Errorf("expected an empty value for a freeform tag, got %q", tag.Value)
		}

		t.Logf("✅ Added freeform tag %s", tag.Key)
	})

	t.Run("duplicate key is rejected with 409", func(t *testing.T) {
		_, err := client.Volumes.AddTag(ctx, volumeID, verda.TagRequest{Key: "environment", Value: "production"})
		if err == nil {
			t.Fatal("expected an error when re-adding an existing key, got nil")
		}

		apiErr, ok := err.(*verda.APIError)
		if !ok {
			t.Fatalf("expected *verda.APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusConflict {
			t.Errorf("expected status 409, got %d", apiErr.StatusCode)
		}

		t.Logf("✅ Duplicate key correctly rejected with %d", apiErr.StatusCode)
	})

	t.Run("tags are returned on the volume", func(t *testing.T) {
		volume, err := client.Volumes.GetVolume(ctx, volumeID)
		if err != nil {
			t.Fatalf("failed to get volume: %v", err)
		}

		if !hasTag(volume.Tags, "environment", "test") {
			t.Errorf("expected environment=test on the volume, got %+v", volume.Tags)
		}
		if !hasTag(volume.Tags, "benchmark", "") {
			t.Errorf("expected the freeform benchmark tag on the volume, got %+v", volume.Tags)
		}
	})

	t.Run("delete tags", func(t *testing.T) {
		for _, key := range []string{"environment", "benchmark", "created-by"} {
			if err := client.Volumes.DeleteTag(ctx, volumeID, key); err != nil {
				t.Errorf("failed to delete tag %s: %v", key, err)
			}
		}

		volume, err := client.Volumes.GetVolume(ctx, volumeID)
		if err != nil {
			t.Fatalf("failed to get volume: %v", err)
		}
		if len(volume.Tags) != 0 {
			t.Errorf("expected no tags left on the volume, got %+v", volume.Tags)
		}

		t.Log("✅ All tags removed")
	})

	t.Run("deleting an unknown tag returns 404", func(t *testing.T) {
		err := client.Volumes.DeleteTag(ctx, volumeID, "does-not-exist")
		if err == nil {
			t.Fatal("expected an error for an unknown tag, got nil")
		}

		apiErr, ok := err.(*verda.APIError)
		if !ok {
			t.Fatalf("expected *verda.APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", apiErr.StatusCode)
		}
	})
}

// hasTag reports whether tags contains a tag with the given key and value
func hasTag(tags []verda.Tag, key, value string) bool {
	for _, tag := range tags {
		if tag.Key == key && tag.Value == value {
			return true
		}
	}
	return false
}
