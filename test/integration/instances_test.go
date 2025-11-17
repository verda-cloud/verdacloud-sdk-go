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

// createTestSSHKey creates a test SSH key and returns its ID
func createTestSSHKey(t *testing.T, client *verda.Client) string {
	ctx := context.Background()
	req := verda.CreateSSHKeyRequest{
		Name:      "integration-test-ssh-key-" + time.Now().Format("20060102-150405"),
		PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC81234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890 test@example.com",
	}

	sshKey, err := client.SSHKeys.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create test SSH key: %v", err)
	}

	return sshKey.ID
}

// createTestStartupScript creates a test startup script and returns its ID
// Returns empty string if startup scripts are not available
func createTestStartupScript(t *testing.T, client *verda.Client) string {
	ctx := context.Background()
	req := verda.CreateStartupScriptRequest{
		Name:   "integration-test-script-" + time.Now().Format("20060102-150405"),
		Script: "#!/bin/bash\n\necho \"Hello from integration test\"\necho \"Test started at: $(date)\" > /tmp/test.log",
	}

	script, err := client.StartupScripts.Create(ctx, req)
	if err != nil {
		// Check if it's a 404 error (not supported on staging)
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
			t.Logf("Startup scripts endpoint not available (404) - skipping startup script creation")
			return ""
		}
		t.Fatalf("failed to create test startup script: %v", err)
	}

	return script.ID
}

// waitForInstanceStatus waits for an instance to reach a specific status
func waitForInstanceStatus(t *testing.T, client *verda.Client, instanceID string, targetStatus string, timeout time.Duration) *verda.Instance {
	ctx := context.Background()
	start := time.Now()
	for time.Since(start) < timeout {
		instance, err := client.Instances.GetByID(ctx, instanceID)
		if err != nil {
			t.Fatalf("failed to get instance %s: %v", instanceID, err)
		}

		t.Logf("Instance %s status: %s (waiting for %s)", instanceID, instance.Status, targetStatus)

		if instance.Status == targetStatus {
			return instance
		}

		// If instance failed, don't continue waiting
		if instance.Status == "FAILED" {
			t.Fatalf("instance %s failed during status wait", instanceID)
		}

		time.Sleep(10 * time.Second)
	}

	t.Fatalf("timeout waiting for instance %s to reach status %s", instanceID, targetStatus)
	return nil
}

// cleanupInstance forcefully cleans up an instance
func cleanupInstance(t *testing.T, client *verda.Client, instanceID string) {
	ctx := context.Background()
	t.Logf("Cleaning up instance %s...", instanceID)

	// First try to get the instance to check its status
	instance, err := client.Instances.GetByID(ctx, instanceID)
	if err != nil {
		t.Logf("Could not get instance %s for cleanup: %v", instanceID, err)
		return
	}

	t.Logf("Instance %s current status: %s", instanceID, instance.Status)

	// If instance is running, try to shut it down first
	if instance.Status == verda.StatusRunning {
		t.Logf("Instance %s is running, stopping it first...", instanceID)
		err = client.Instances.Shutdown(ctx, instanceID)
		if err != nil {
			t.Logf("Failed to shutdown instance %s: %v", instanceID, err)
		} else {
			// Wait for shutdown with shorter timeout for cleanup
			for i := 0; i < 30; i++ {
				instance, err := client.Instances.GetByID(ctx, instanceID)
				if err == nil && instance.Status == verda.StatusOffline {
					t.Logf("Instance %s successfully shut down", instanceID)
					break
				}
				time.Sleep(10 * time.Second)
			}
		}
	}

	// If instance is pending/deploying, wait a bit for it to reach a stable state
	if instance.Status == verda.StatusPending {
		t.Logf("Instance %s is pending, waiting for stable state...", instanceID)
		time.Sleep(30 * time.Second)
	}

	// Now try to delete the instance
	err = client.Instances.Delete(ctx, instanceID, nil)
	if err != nil {
		// Don't fail the test on cleanup errors, just log them
		t.Logf("Warning: Failed to delete instance %s: %v (this is non-fatal for test cleanup)", instanceID, err)
	} else {
		t.Logf("Successfully initiated deletion of instance %s", instanceID)
	}
}

// TestListInstances_Integration tests listing instances
func TestListInstances_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)

	ctx := context.Background()
	instances, err := client.Instances.Get(ctx, "")
	if err != nil {
		t.Fatalf("failed to list instances: %v", err)
	}

	t.Logf("Found %d instances", len(instances))

	// Validate response structure
	for i, inst := range instances {
		if inst.ID == "" {
			t.Errorf("instance %d has empty ID", i)
		}
		if inst.InstanceType == "" {
			t.Errorf("instance %d has empty InstanceType", i)
		}
		if inst.Status == "" {
			t.Errorf("instance %d has empty Status", i)
		}

		t.Logf("Instance %d: ID=%s, Type=%s, Status=%s, Location=%s",
			i, inst.ID, inst.InstanceType, inst.Status, inst.Location)
	}

	// Test instance availability
	available, err := client.Instances.IsAvailable(ctx, "1V100.6V", false, "")
	if err != nil {
		t.Errorf("failed to check availability: %v", err)
	}
	t.Logf("1V100.6V available: %v", available)
}

// TestRateLimiting_Integration tests how the service handles rate limiting
func TestRateLimiting_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)

	ctx := context.Background()
	// Make multiple rapid requests to test rate limiting
	const numRequests = 10
	errors := make([]error, numRequests)

	for i := 0; i < numRequests; i++ {
		_, errors[i] = client.Instances.Get(ctx, "")
		if i < numRequests-1 {
			time.Sleep(100 * time.Millisecond) // Small delay
		}
	}

	errorCount := 0
	for _, err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Request error (expected for rate limiting): %v", err)
		}
	}

	// We expect some requests might fail due to rate limiting
	if errorCount == numRequests {
		t.Error("all requests failed, this might indicate a bigger issue")
	}

	t.Logf("Rate limiting test: %d/%d requests succeeded", numRequests-errorCount, numRequests)
}

// TestCreateAndDeleteInstance_Integration tests the full lifecycle of creating and deleting an instance
func TestCreateAndDeleteInstance_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)

	ctx := context.Background()
	// Create test resources
	sshKeyID := createTestSSHKey(t, client)
	scriptID := createTestStartupScript(t, client)

	// Create instance with minimal configuration
	input := verda.CreateInstanceRequest{
		InstanceType: "1V100.6V",
		Image:        "ubuntu-24.04-cuda-12.8-open-docker",
		SSHKeyIDs:    []string{sshKeyID},
		LocationCode: verda.LocationFIN01,
		Hostname:     "integration-test-vm",
		Description:  "Integration test instance - safe to delete",
	}

	// Add startup script only if it was created successfully
	if scriptID != "" {
		input.StartupScriptID = &scriptID
	}

	// Check availability first
	available, err := client.Instances.IsAvailable(ctx, input.InstanceType, input.IsSpot, input.LocationCode)
	if err != nil {
		t.Fatalf("failed to check instance availability: %v", err)
	}
	if !available {
		t.Skipf("Instance type %s not available in location %s", input.InstanceType, input.LocationCode)
	}

	instance, err := client.Instances.Create(ctx, input)
	if err != nil {
		// On staging, instance creation can intermittently return 5xx
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode >= 500 {
			t.Skipf("skipping instance creation due to server error: %v", apiErr)
			return
		}
		// Handle quota/limit errors gracefully
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 400 {
			if strings.Contains(apiErr.Message, "limit exceeded") || strings.Contains(apiErr.Message, "quota") {
				t.Skipf("Skipping instance creation due to quota: %v", apiErr)
				return
			}
		}
		t.Fatalf("failed to create instance: %v", err)
	}

	if instance.ID == "" {
		t.Fatal("created instance has empty ID")
	}

	t.Logf("Created instance with ID: %s", instance.ID)

	// Cleanup: Delete the instance
	defer func() {
		cleanupInstance(t, client, instance.ID)

		// Cleanup test resources
		ctx := context.Background()
		err := client.SSHKeys.Delete(ctx, sshKeyID)
		if err != nil {
			t.Errorf("failed to delete test SSH key %s: %v", sshKeyID, err)
		} else {
			t.Log("Successfully cleaned up test SSH key")
		}

		// Only try to delete startup script if it was created
		if scriptID != "" {
			err = client.StartupScripts.Delete(ctx, scriptID)
			if err != nil {
				t.Errorf("failed to delete test startup script %s: %v", scriptID, err)
			} else {
				t.Log("Successfully cleaned up test startup script")
			}
		}
	}()

	// Wait a moment for instance to be created
	time.Sleep(5 * time.Second)

	// Verify instance appears in list
	instances, err := client.Instances.Get(ctx, "")
	if err != nil {
		t.Fatalf("failed to list instances: %v", err)
	}

	found := false
	for _, inst := range instances {
		if inst.ID == instance.ID {
			found = true
			if inst.Status == "" {
				t.Error("instance status is empty")
			}
			if inst.InstanceType != input.InstanceType {
				t.Errorf("expected instance type %s, got %s", input.InstanceType, inst.InstanceType)
			}
			if inst.Hostname != input.Hostname {
				t.Errorf("expected hostname %s, got %s", input.Hostname, inst.Hostname)
			}
			break
		}
	}

	if !found {
		t.Errorf("created instance %s not found in list", instance.ID)
	}

	// Test getting instance by ID
	retrievedInstance, err := client.Instances.GetByID(ctx, instance.ID)
	if err != nil {
		t.Fatalf("failed to get instance by ID: %v", err)
	}

	if retrievedInstance.ID != instance.ID {
		t.Errorf("expected instance ID %s, got %s", instance.ID, retrievedInstance.ID)
	}
}
