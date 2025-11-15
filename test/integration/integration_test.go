//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// TestIntegration runs integration tests against the real Verda API
// These tests require real API credentials and will create actual resources
// Run with: go test -tags=integration ./test/integration
func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	// Get credentials and configuration from environment
	clientID := os.Getenv("VERDA_CLIENT_ID")
	clientSecret := os.Getenv("VERDA_CLIENT_SECRET")
	baseURL := os.Getenv("VERDA_BASE_URL") // Optional: defaults to production

	if clientID == "" || clientSecret == "" {
		t.Skip("VERDA_CLIENT_ID and VERDA_CLIENT_SECRET must be set for integration tests")
	}

	// Create client with optional base URL override
	options := []verda.ClientOption{
		verda.WithClientID(clientID),
		verda.WithClientSecret(clientSecret),
	}
	if baseURL != "" {
		options = append(options, verda.WithBaseURL(baseURL))
		t.Logf("Using custom base URL: %s", baseURL)
	}

	client, err := verda.NewClient(options...)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	t.Run("authentication", func(t *testing.T) {
		// Test that we can authenticate
		token, err := client.Auth.GetValidToken()
		if err != nil {
			t.Errorf("authentication failed: %v", err)
		}
		if token == nil || token.AccessToken == "" {
			t.Error("expected valid access token")
		}
	})

	t.Run("balance", func(t *testing.T) {
		ctx := context.Background()
		balance, err := client.Balance.Get(ctx)
		if err != nil {
			t.Errorf("failed to get balance: %v", err)
		}
		if balance == nil {
			t.Error("expected balance information")
		}
		t.Logf("Account balance: %.2f %s", balance.Amount, balance.Currency)
	})

	t.Run("locations", func(t *testing.T) {
		ctx := context.Background()
		locations, err := client.Locations.Get(ctx)
		if err != nil {
			t.Errorf("failed to get locations: %v", err)
		}
		if len(locations) == 0 {
			t.Error("expected at least one location")
		}
		t.Logf("Found %d locations", len(locations))
	})

	t.Run("ssh_keys", func(t *testing.T) {
		ctx := context.Background()
		keys, err := client.SSHKeys.Get(ctx)
		if err != nil {
			t.Errorf("failed to get SSH keys: %v", err)
		}
		t.Logf("Found %d SSH keys", len(keys))
	})

	t.Run("instances", func(t *testing.T) {
		ctx := context.Background()
		instances, err := client.Instances.Get(ctx, "")
		if err != nil {
			t.Errorf("failed to get instances: %v", err)
		}
		t.Logf("Found %d instances", len(instances))

		// Test instance availability
		available, err := client.Instances.IsAvailable(ctx, "1V100.6V", false, "")
		if err != nil {
			t.Errorf("failed to check availability: %v", err)
		}
		t.Logf("1V100.6V available: %v", available)
	})

	t.Run("volumes", func(t *testing.T) {
		ctx := context.Background()
		volumes, err := client.Volumes.Get(ctx)
		if err != nil {
			t.Errorf("failed to get volumes: %v", err)
		}
		t.Logf("Found %d volumes", len(volumes))
	})

	t.Run("startup_scripts", func(t *testing.T) {
		ctx := context.Background()
		scripts, err := client.StartupScripts.Get(ctx)
		if err != nil {
			// Check if it's a 404 error (not supported on staging)
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Logf("Startup scripts endpoint not available (404) - skipping test")
				return
			}
			t.Errorf("failed to get startup scripts: %v", err)
		}
		t.Logf("Found %d startup scripts", len(scripts))
	})

	t.Run("containers", func(t *testing.T) {
		ctx := context.Background()
		containers, err := client.Containers.Get(ctx)
		if err != nil {
			// Check if it's a 404 error (not supported on staging)
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Logf("Containers endpoint not available (404) - skipping test")
				return
			}
			t.Errorf("failed to get containers: %v", err)
		}
		t.Logf("Found %d containers", len(containers))
	})
}
