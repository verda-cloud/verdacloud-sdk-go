//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestContainerTypesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get container types", func(t *testing.T) {
		containerTypes, err := client.ContainerTypes.Get(ctx, "usd")
		if err != nil {
			// Check if it's a 404 error (not supported on staging)
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Container types endpoint not available (404)")
				return
			}
			t.Errorf("failed to get container types: %v", err)
		}
		t.Logf("Found %d container types", len(containerTypes))
	})
}
