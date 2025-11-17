//go:build integration
// +build integration

package integration

import (
	"testing"
)

func TestAuthIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)

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
