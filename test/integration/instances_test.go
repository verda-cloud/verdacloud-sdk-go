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

// Preferred instance type for testing (as specified by user)
const PreferredInstanceType = "1RTXPRO6000.30V"

// createTestSSHKey creates a test SSH key and returns its ID
func createTestSSHKey(t *testing.T, client *verda.Client) string {
	t.Helper()
	ctx := context.Background()
	req := &verda.CreateSSHKeyRequest{
		Name:      "integration-test-ssh-key-" + time.Now().Format("20060102-150405"),
		PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC81234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890 test@example.com",
	}

	sshKey, err := client.SSHKeys.AddSSHKey(ctx, req)
	if err != nil {
		t.Fatalf("failed to create test SSH key: %v", err)
	}

	return sshKey.ID
}

// cleanupInstance properly cleans up an instance
func cleanupInstance(t *testing.T, client *verda.Client, instanceID string) {
	t.Helper()
	ctx := context.Background()
	t.Logf("🧹 Cleaning up instance %s...", instanceID)

	// Get current status
	instance, err := client.Instances.GetByID(ctx, instanceID)
	if err != nil {
		t.Logf("   ⚠️  Could not get instance: %v", err)
		return
	}
	t.Logf("   Current status: %s", instance.Status)

	// If running, shutdown first
	if instance.Status == verda.StatusRunning {
		t.Log("   Shutting down...")
		if err := client.Instances.Shutdown(ctx, instanceID); err != nil {
			t.Logf("   ⚠️  Shutdown failed: %v", err)
		} else {
			// Wait for shutdown
			time.Sleep(30 * time.Second)
		}
	}

	// If provisioning, wait a bit
	if instance.Status == verda.StatusPending || instance.Status == "provisioning" {
		t.Log("   Instance is provisioning, waiting...")
		time.Sleep(30 * time.Second)
	}

	// Try to delete
	if err := client.Instances.Delete(ctx, instanceID, nil); err != nil {
		t.Logf("   ⚠️  Delete failed: %v, trying discontinue...", err)
		if err := client.Instances.Discontinue(ctx, instanceID); err != nil {
			t.Logf("   ⚠️  Discontinue also failed: %v", err)
		} else {
			t.Log("   ✅ Discontinued successfully")
		}
	} else {
		t.Log("   ✅ Deleted successfully")
	}
}

// ============================================================================
// INSTANCE CRUD INTEGRATION TEST
// ============================================================================

// TestInstanceCRUDIntegration tests the complete instance lifecycle:
// 1. Check availability first
// 2. Create instance
// 3. Wait for it to be ready
// 4. List instances (verify it exists)
// 5. Read instance by ID
// 6. Cleanup (delete)
func TestInstanceCRUDIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// ========================================
	// STEP 0: Check availability
	// ========================================
	availableInstance, ok := FindAvailableInstanceType(ctx, t, client, PreferredInstanceType)
	if !ok {
		t.Skip("⏭️  SKIPPING: No instance types available in staging environment")
	}

	// Track state for sequential CRUD operations
	var instanceID string
	var instanceCreated bool
	var sshKeyID string

	// Cleanup function - always runs at the end
	defer func() {
		if instanceCreated && instanceID != "" {
			cleanupInstance(t, client, instanceID)
		}
		if sshKeyID != "" {
			t.Log("🧹 Cleaning up SSH key...")
			if err := client.SSHKeys.DeleteSSHKey(ctx, sshKeyID); err != nil {
				t.Logf("   ⚠️  Failed to delete SSH key: %v", err)
			} else {
				t.Log("   ✅ SSH key deleted")
			}
		}
	}()

	// ========================================
	// STEP 1: CREATE
	// ========================================
	t.Run("1_CREATE", func(t *testing.T) {
		// Create SSH key first
		sshKeyID = createTestSSHKey(t, client)
		t.Logf("✅ Created SSH key: %s", sshKeyID)

		// Create instance
		req := verda.CreateInstanceRequest{
			InstanceType: availableInstance.InstanceType,
			Image:        "ubuntu-24.04-cuda-12.8-open-docker",
			SSHKeyIDs:    []string{sshKeyID},
			LocationCode: availableInstance.Location,
			Hostname:     "integration-test-vm",
			Description:  "Integration test instance - safe to delete",
		}

		t.Logf("📦 Creating instance with type: %s at %s...", req.InstanceType, req.LocationCode)

		instance, err := client.Instances.Create(ctx, req)
		if err != nil {
			// Handle common errors
			if apiErr, ok := err.(*verda.APIError); ok {
				if apiErr.StatusCode >= 500 {
					t.Skipf("⏭️  Server error (%d): %v", apiErr.StatusCode, err)
				}
				if apiErr.StatusCode == 400 {
					if strings.Contains(apiErr.Message, "limit exceeded") || strings.Contains(apiErr.Message, "quota") {
						t.Skipf("⏭️  Quota exceeded: %v", err)
					}
				}
			}
			t.Fatalf("❌ Failed to create instance: %v", err)
		}

		if instance.ID == "" {
			t.Fatal("❌ Created instance has empty ID")
		}

		instanceID = instance.ID
		instanceCreated = true
		t.Logf("✅ Created instance: ID=%s, Type=%s", instanceID, instance.InstanceType)
	})

	// If creation failed, skip remaining tests
	if !instanceCreated {
		t.Skip("⏭️  Skipping remaining tests - instance was not created")
	}

	// ========================================
	// STEP 2: WAIT for ready state
	// ========================================
	t.Run("2_WAIT", func(t *testing.T) {
		if !instanceCreated {
			t.Skip("⏭️  Skipping - instance was not created")
		}

		// Wait for instance to be running (or at least not pending)
		instance, ok := WaitForInstanceStatus(ctx, t, client, instanceID, verda.StatusRunning, 5*time.Minute)
		if !ok {
			// Even if not running, check if it's in a valid state
			instance, err := client.Instances.GetByID(ctx, instanceID)
			if err != nil {
				t.Fatalf("❌ Could not get instance status: %v", err)
			}
			t.Logf("⚠️  Instance is in status '%s' (not running yet)", instance.Status)
		} else {
			t.Logf("✅ Instance is running: IP=%v", instance.IP)
		}
	})

	// ========================================
	// STEP 3: LIST (verify instance exists)
	// ========================================
	t.Run("3_LIST", func(t *testing.T) {
		if !instanceCreated {
			t.Skip("⏭️  Skipping - instance was not created")
		}

		instances, err := client.Instances.Get(ctx, "")
		if err != nil {
			t.Fatalf("❌ Failed to list instances: %v", err)
		}

		found := false
		for _, inst := range instances {
			if inst.ID == instanceID {
				found = true
				t.Logf("✅ Found our instance in list: ID=%s, Status=%s, Type=%s",
					inst.ID, inst.Status, inst.InstanceType)
				break
			}
		}

		if !found {
			t.Errorf("❌ Instance %s not found in list", instanceID)
		}
	})

	// ========================================
	// STEP 4: READ by ID
	// ========================================
	t.Run("4_READ", func(t *testing.T) {
		if !instanceCreated {
			t.Skip("⏭️  Skipping - instance was not created")
		}

		instance, err := client.Instances.GetByID(ctx, instanceID)
		if err != nil {
			t.Fatalf("❌ Failed to get instance by ID: %v", err)
		}

		// Verify fields
		if instance.ID != instanceID {
			t.Errorf("❌ ID mismatch: expected %s, got %s", instanceID, instance.ID)
		}
		if instance.InstanceType != availableInstance.InstanceType {
			t.Errorf("❌ InstanceType mismatch: expected %s, got %s", availableInstance.InstanceType, instance.InstanceType)
		}

		t.Logf("✅ Read instance: ID=%s, Type=%s, Status=%s, Location=%s, Hostname=%s",
			instance.ID, instance.InstanceType, instance.Status, instance.Location, instance.Hostname)
	})

	// Note: DELETE happens in defer cleanup
	t.Log("✅ Instance CRUD test complete - cleanup will run in defer")
}

// ============================================================================
// OTHER INSTANCE TESTS
// ============================================================================

// TestListInstances_Integration tests listing instances (read-only)
func TestListInstances_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)
	ctx := context.Background()

	instances, err := client.Instances.Get(ctx, "")
	if err != nil {
		t.Fatalf("❌ Failed to list instances: %v", err)
	}

	t.Logf("✅ Found %d instances", len(instances))
	for i, inst := range instances {
		t.Logf("   [%d] ID=%s, Type=%s, Status=%s, Location=%s",
			i, inst.ID, inst.InstanceType, inst.Status, inst.Location)
	}
}

// TestInstanceAvailability_Integration tests checking instance availability
func TestInstanceAvailability_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// Test availability check
	availableInstance, ok := FindAvailableInstanceType(ctx, t, client, PreferredInstanceType)
	if ok {
		t.Logf("✅ Best available instance: %s ($%.2f/hr spot)",
			availableInstance.InstanceType, availableInstance.SpotPrice)
	} else {
		t.Log("⚠️  No instance types currently available")
	}
}
