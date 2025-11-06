//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verda-go/pkg/verda"
)

func TestSSHKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := createTestClient(t)

	t.Run("get_ssh_keys", func(t *testing.T) {
		ctx := context.Background()
		keys, err := client.SSHKeys.Get(ctx)
		if err != nil {
			t.Errorf("failed to get SSH keys: %v", err)
		}
		t.Logf("Found %d SSH keys", len(keys))
	})
}

// TestSSHKeyLifecycle tests creating and managing SSH keys
func TestSSHKeyLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := createTestClient(t)

	var keyID string

	// Test public key (this is a dummy key for testing)
	testPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7vbqajDjnmXitxjHa8YNas6RMk+JgTwDOQP1J3TN9x+3X9C7v+RzF8Z8X+QpK5M1J8q8Y2lEt3vDgIa4V7VZyP+qJz7Ft9+CqZZx8F2J4DqF5F6Y8L4Y9I+e9oI3D5y0K4Y9I+e9oI3D5y0 test@example.com"

	t.Run("create_ssh_key", func(t *testing.T) {
		ctx := context.Background()
		req := verda.CreateSSHKeyRequest{
			Name:      "test-go-sdk-key",
			PublicKey: testPublicKey,
		}

		key, err := client.SSHKeys.Create(ctx, req)
		if err != nil {
			t.Fatalf("failed to create SSH key: %v", err)
		}

		keyID = key.ID
		t.Logf("Created SSH key: %s (%s)", key.Name, key.ID)

		if key.Name != req.Name {
			t.Errorf("expected key name %s, got %s", req.Name, key.Name)
		}
	})

	t.Run("get_ssh_key", func(t *testing.T) {
		if keyID == "" {
			t.Skip("no key ID from previous test")
		}

		ctx := context.Background()
		key, err := client.SSHKeys.GetByID(ctx, keyID)
		if err != nil {
			t.Errorf("failed to get SSH key: %v", err)
		}
		if key == nil {
			t.Error("expected SSH key, got nil")
		}
		if key != nil {
			t.Logf("SSH key fingerprint: %s", key.Fingerprint)
		}
	})

	// Clean up - delete the SSH key
	t.Run("cleanup_ssh_key", func(t *testing.T) {
		if keyID == "" {
			t.Skip("no key ID to clean up")
		}

		ctx := context.Background()
		err := client.SSHKeys.Delete(ctx, keyID)
		if err != nil {
			t.Errorf("failed to delete SSH key %s: %v", keyID, err)
		} else {
			t.Logf("Successfully deleted SSH key: %s", keyID)
		}
	})
}
