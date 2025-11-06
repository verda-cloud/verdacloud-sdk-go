//go:build integration
// +build integration

package integration

import (
	"os"
	"testing"

	"github.com/verda-cloud/verda-go/pkg/verda"
)

func TestAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := createTestClient(t)

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
}

// createTestClient creates a client for integration tests
func createTestClient(t *testing.T) *verda.Client {
	clientID := os.Getenv("VERDA_CLIENT_ID")
	clientSecret := os.Getenv("VERDA_CLIENT_SECRET")
	baseURL := os.Getenv("VERDA_BASE_URL") // Optional: defaults to production

	if clientID == "" || clientSecret == "" {
		t.Skip("VERDA_CLIENT_ID and VERDA_CLIENT_SECRET must be set for integration tests")
	}

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

	return client
}
