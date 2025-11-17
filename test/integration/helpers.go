//go:build integration
// +build integration

package integration

import (
	"os"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// getTestClient creates a client for integration tests using environment variables
func getTestClient(t *testing.T) *verda.Client {
	t.Helper()

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

	return client
}
