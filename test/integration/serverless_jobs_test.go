//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestServerlessJobsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get job deployments", func(t *testing.T) {
		jobs, err := client.ServerlessJobs.GetJobDeployments(ctx)
		if err != nil {
			// Check if this is a 404 - endpoint might not exist in staging
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Job deployments endpoint not available (404)")
				return
			}
			t.Errorf("failed to get job deployments: %v", err)
		}
		t.Logf("Found %d job deployments", len(jobs))
	})
}
