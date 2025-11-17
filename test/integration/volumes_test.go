//go:build integration
// +build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// cleanupTestVolumes removes all test volumes created during integration tests
// This should be called at the beginning of volume tests to ensure a clean state
func cleanupTestVolumes(t *testing.T, client *verda.Client) {
	ctx := context.Background()
	t.Log("Checking for existing test volumes to cleanup...")

	volumes, err := client.Volumes.Get(ctx)
	if err != nil {
		t.Logf("Warning: failed to list volumes for cleanup: %v", err)
		return
	}

	testVolumePrefixes := []string{
		"integration-test-",
		"test-volume-",
	}

	cleanedCount := 0
	for _, volume := range volumes {
		// Check if this is a test volume
		isTestVolume := false
		for _, prefix := range testVolumePrefixes {
			if strings.HasPrefix(volume.Name, prefix) {
				isTestVolume = true
				break
			}
		}

		if isTestVolume {
			t.Logf("Cleaning up existing test volume: %s (ID: %s, Status: %s)", volume.Name, volume.ID, volume.Status)
			err := client.Volumes.Delete(ctx, volume.ID, true)
			if err != nil {
				t.Logf("Warning: failed to delete volume %s: %v", volume.ID, err)
			} else {
				cleanedCount++
			}
		}
	}

	if cleanedCount > 0 {
		t.Logf("Cleaned up %d existing test volume(s)", cleanedCount)
		// Give the API a moment to process the deletions
		time.Sleep(2 * time.Second)
	} else {
		t.Log("No existing test volumes found to cleanup")
	}
}

// cleanupVolume forcefully cleans up a single volume
func cleanupVolume(t *testing.T, client *verda.Client, volumeID string) {
	ctx := context.Background()
	t.Logf("Cleaning up volume %s...", volumeID)

	err := client.Volumes.Delete(ctx, volumeID, true)
	if err != nil {
		// Don't fail the test on cleanup errors, just log them
		t.Logf("Warning: Failed to delete volume %s: %v (this is non-fatal for test cleanup)", volumeID, err)
	} else {
		t.Logf("Successfully initiated deletion of volume %s", volumeID)
	}
}

func TestVolumes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := createTestClient(t)

	t.Run("get_volumes", func(t *testing.T) {
		ctx := context.Background()
		volumes, err := client.Volumes.Get(ctx)
		if err != nil {
			t.Errorf("failed to get volumes: %v", err)
		}
		t.Logf("Found %d volumes", len(volumes))
	})
}

// TestListVolumes_Integration tests listing volumes with status filtering
func TestListVolumes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createTestClient(t)

	ctx := context.Background()
	// Test listing volumes with specific status
	volumes, err := client.Volumes.GetByStatus(ctx, verda.VolumeStatusOrdered)
	if err != nil {
		t.Fatalf("failed to list volumes with status 'ordered': %v", err)
	}

	t.Logf("Found %d volumes with status 'ordered'", len(volumes))

	// Test listing all volumes (ignore status)
	volumes, err = client.Volumes.Get(ctx)
	if err != nil {
		t.Fatalf("failed to list all volumes: %v", err)
	}

	t.Logf("Found %d total volumes", len(volumes))

	// Validate response structure
	for i, vol := range volumes {
		if vol.ID == "" {
			t.Errorf("volume %d has empty ID", i)
		}
		if vol.Name == "" {
			t.Errorf("volume %d has empty Name", i)
		}
		if vol.Status == "" {
			t.Errorf("volume %d has empty Status", i)
		}
		if vol.Type == "" {
			t.Errorf("volume %d has empty Type", i)
		}

		t.Logf("Volume %d: ID=%s, Name=%s, Status=%s, Type=%s, Size=%d",
			i, vol.ID, vol.Name, vol.Status, vol.Type, vol.Size)
	}
}

// TestCreateVolume_Integration tests creating a volume
func TestCreateVolume_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createTestClient(t)

	// Cleanup any existing test volumes first
	cleanupTestVolumes(t, client)

	ctx := context.Background()
	volumeID, err := client.Volumes.Create(ctx, verda.VolumeCreateRequest{
		Type:     verda.VolumeTypeNVMe,
		Location: verda.LocationFIN01,
		Size:     50,
		Name:     "integration-test-volume",
	})

	if err != nil {
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 400 && (strings.Contains(apiErr.Message, "Volume limit exceeded") || strings.Contains(apiErr.Message, "Storage limit exceeded")) {
			t.Skipf("Skipping volume create due to quota: %v", apiErr)
			return
		}
		t.Fatalf("failed to create volume: %v", err)
	}

	if volumeID == "" {
		t.Fatal("created volume has empty ID")
	}

	t.Logf("Created volume with ID: %s", volumeID)

	// Cleanup
	defer cleanupVolume(t, client, volumeID)

	// Wait a moment for volume to be created
	time.Sleep(3 * time.Second)

	// Verify volume appears in list
	volumes, err := client.Volumes.Get(ctx)
	if err != nil {
		t.Fatalf("failed to list volumes: %v", err)
	}

	found := false
	for _, vol := range volumes {
		if vol.ID == volumeID {
			found = true
			if vol.Name != "integration-test-volume" {
				t.Errorf("expected volume name 'integration-test-volume', got %s", vol.Name)
			}
			if vol.Type != "NVMe" {
				t.Errorf("expected volume type 'NVMe', got %s", vol.Type)
			}
			if vol.Size != 50 {
				t.Errorf("expected volume size 50, got %d", vol.Size)
			}
			t.Logf("Found volume: ID=%s, Name=%s, Status=%s, Type=%s, Size=%d",
				vol.ID, vol.Name, vol.Status, vol.Type, vol.Size)
			break
		}
	}

	if !found {
		t.Errorf("created volume %s not found in list", volumeID)
	}

	// Test getting volume by ID
	volume, err := client.Volumes.GetByID(ctx, volumeID)
	if err != nil {
		t.Fatalf("failed to get volume by ID: %v", err)
	}

	if volume.ID != volumeID {
		t.Errorf("expected volume ID %s, got %s", volumeID, volume.ID)
	}
	if volume.Name != "integration-test-volume" {
		t.Errorf("expected volume name 'integration-test-volume', got %s", volume.Name)
	}
}

// TestVolumeLifecycle_Integration tests the full lifecycle of volumes
func TestVolumeLifecycle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createTestClient(t)

	// Cleanup any existing test volumes first
	cleanupTestVolumes(t, client)

	ctx := context.Background()

	// Test creating multiple volumes with different configurations
	volumeConfigs := []struct {
		name       string
		volumeType string
		size       int
	}{
		{
			name:       "test-volume-nvme",
			volumeType: verda.VolumeTypeNVMe,
			size:       50,
		},
		{
			name:       "test-volume-hdd",
			volumeType: verda.VolumeTypeHDD,
			size:       100,
		},
	}

	var createdVolumeIDs []string

	// Create volumes
	for _, config := range volumeConfigs {
		volumeID, err := client.Volumes.Create(ctx, verda.VolumeCreateRequest{
			Type:     config.volumeType,
			Location: verda.LocationFIN01,
			Size:     config.size,
			Name:     config.name,
		})
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 400 && (strings.Contains(apiErr.Message, "Volume limit exceeded") || strings.Contains(apiErr.Message, "Storage limit exceeded")) {
				t.Skipf("Skipping volume lifecycle due to quota: %v", apiErr)
				return
			}
			t.Fatalf("failed to create volume %s: %v", config.name, err)
		}

		createdVolumeIDs = append(createdVolumeIDs, volumeID)
		t.Logf("Created volume '%s' with ID: %s", config.name, volumeID)
	}

	// Wait for volumes to be created
	time.Sleep(5 * time.Second)

	// Verify all volumes exist in the list
	allVolumes, err := client.Volumes.Get(ctx)
	if err != nil {
		t.Fatalf("failed to list volumes: %v", err)
	}

	for i, volumeID := range createdVolumeIDs {
		found := false
		for _, volume := range allVolumes {
			if volume.ID == volumeID {
				found = true
				if volume.Name != volumeConfigs[i].name {
					t.Errorf("expected volume name '%s', got '%s'", volumeConfigs[i].name, volume.Name)
				}
				if volume.Type != volumeConfigs[i].volumeType {
					t.Errorf("expected volume type '%s', got '%s'", volumeConfigs[i].volumeType, volume.Type)
				}
				break
			}
		}
		if !found {
			t.Errorf("created volume %s not found in list", volumeID)
		}
	}

	t.Logf("All %d created volumes found in list", len(createdVolumeIDs))

	// Cleanup all created volumes
	defer func() {
		for i, volumeID := range createdVolumeIDs {
			t.Logf("Cleaning up volume %s (%s)...", volumeConfigs[i].name, volumeID)
			cleanupVolume(t, client, volumeID)
		}
	}()

	// Test volume status filtering with valid API status values
	for _, status := range []string{
		verda.VolumeStatusOrdered,
		verda.VolumeStatusAttached,
		verda.VolumeStatusAttaching,
		verda.VolumeStatusDetached,
		verda.VolumeStatusCreated,
	} {
		volumes, err := client.Volumes.GetByStatus(ctx, status)
		if err != nil {
			t.Errorf("failed to list volumes with status '%s': %v", status, err)
		} else {
			t.Logf("Found %d volumes with status '%s'", len(volumes), status)
		}
	}
}
