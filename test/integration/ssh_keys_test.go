//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestSSHKeysIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)

	t.Run("get all SSH keys", func(t *testing.T) {
		ctx := context.Background()
		keys, err := client.SSHKeys.GetAllSSHKeys(ctx)
		if err != nil {
			t.Errorf("failed to get SSH keys: %v", err)
		}
		t.Logf("Found %d SSH keys", len(keys))

		// Verify structure if keys exist
		if len(keys) > 0 {
			for i, key := range keys {
				if i < 3 { // Log first 3
					t.Logf("SSH Key: %s (%s) - Fingerprint: %s",
						key.Name, key.ID, key.Fingerprint)
				}
				if key.ID == "" {
					t.Errorf("key %d missing ID", i)
				}
				if key.Name == "" {
					t.Errorf("key %d missing Name", i)
				}
			}
		}
	})

	t.Run("test deprecated Get method", func(t *testing.T) {
		ctx := context.Background()
		keys, err := client.SSHKeys.Get(ctx)
		if err != nil {
			t.Errorf("failed to get SSH keys with deprecated method: %v", err)
		}
		t.Logf("Deprecated Get method returned %d SSH keys", len(keys))
	})
}

// TestSSHKeyLifecycleIntegration tests creating and managing SSH keys
func TestSSHKeyLifecycleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)

	var keyID string

	// Test public key (this is a dummy key for testing)
	testPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7vbqajDjnmXitxjHa8YNas6RMk+JgTwDOQP1J3TN9x+3X9C7v+RzF8Z8X+QpK5M1J8q8Y2lEt3vDgIa4V7VZyP+qJz7Ft9+CqZZx8F2J4DqF5F6Y8L4Y9I+e9oI3D5y0K4Y9I+e9oI3D5y0 test@example.com"

	t.Run("create SSH key", func(t *testing.T) {
		ctx := context.Background()
		req := &verda.CreateSSHKeyRequest{
			Name:      "test-go-sdk-key",
			PublicKey: testPublicKey,
		}

		key, err := client.SSHKeys.AddSSHKey(ctx, req)
		if err != nil {
			t.Fatalf("failed to create SSH key: %v", err)
		}

		keyID = key.ID
		t.Logf("Created SSH key: %s (%s)", key.Name, key.ID)

		if key.Name != req.Name {
			t.Errorf("expected key name %s, got %s", req.Name, key.Name)
		}
	})

	t.Run("get SSH key by ID", func(t *testing.T) {
		if keyID == "" {
			t.Skip("no key ID from previous test")
		}

		ctx := context.Background()
		key, err := client.SSHKeys.GetSSHKeyByID(ctx, keyID)
		if err != nil {
			t.Errorf("failed to get SSH key: %v", err)
		}
		if key == nil {
			t.Error("expected SSH key, got nil")
		}
		if key != nil {
			t.Logf("SSH key fingerprint: %s", key.Fingerprint)

			if key.ID != keyID {
				t.Errorf("expected key ID %s, got %s", keyID, key.ID)
			}
		}
	})

	// Clean up - delete the SSH key
	t.Run("delete SSH key", func(t *testing.T) {
		if keyID == "" {
			t.Skip("no key ID to clean up")
		}

		ctx := context.Background()
		err := client.SSHKeys.DeleteSSHKey(ctx, keyID)
		if err != nil {
			t.Errorf("failed to delete SSH key %s: %v", keyID, err)
		} else {
			t.Logf("Successfully deleted SSH key: %s", keyID)
		}
	})
}

// TestSSHKeyMultipleDeleteIntegration tests deleting multiple SSH keys at once
func TestSSHKeyMultipleDeleteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// Test public key (this is a dummy key for testing)
	testPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7vbqajDjnmXitxjHa8YNas6RMk+JgTwDOQP1J3TN9x+3X9C7v+RzF8Z8X+QpK5M1J8q8Y2lEt3vDgIa4V7VZyP+qJz7Ft9+CqZZx8F2J4DqF5F6Y8L4Y9I+e9oI3D5y0K4Y9I+e9oI3D5y0 test@example.com"

	var keyIDs []string

	// Create multiple test keys
	t.Run("create multiple SSH keys", func(t *testing.T) {
		for i := 1; i <= 2; i++ {
			req := &verda.CreateSSHKeyRequest{
				Name:      fmt.Sprintf("test-go-sdk-multi-key-%d", i),
				PublicKey: testPublicKey,
			}

			key, err := client.SSHKeys.AddSSHKey(ctx, req)
			if err != nil {
				t.Errorf("failed to create SSH key %d: %v", i, err)
				continue
			}

			keyIDs = append(keyIDs, key.ID)
			t.Logf("Created SSH key %d: %s (%s)", i, key.Name, key.ID)
		}
	})

	// Delete all created keys at once
	t.Run("delete multiple SSH keys", func(t *testing.T) {
		if len(keyIDs) == 0 {
			t.Skip("no keys to delete")
		}

		err := client.SSHKeys.DeleteMultipleSSHKeys(ctx, keyIDs)
		if err != nil {
			t.Errorf("failed to delete multiple SSH keys: %v", err)
		} else {
			t.Logf("Successfully deleted %d SSH keys", len(keyIDs))
		}
	})
}
