//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestContainers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := createTestClient(t)

	t.Run("get_containers", func(t *testing.T) {
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
