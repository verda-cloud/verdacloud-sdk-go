//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestContainerDeploymentsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get deployments", func(t *testing.T) {
		deployments, err := client.ContainerDeployments.GetDeployments(ctx)
		if err != nil {
			// Check if it's a 404 error (not supported on staging)
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Container deployments endpoint not available (404)")
				return
			}
			t.Errorf("failed to get container deployments: %v", err)
		}
		t.Logf("Found %d container deployments", len(deployments))
	})

	t.Run("get compute resources", func(t *testing.T) {
		resources, err := client.ContainerDeployments.GetServerlessComputeResources(ctx)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Serverless compute resources endpoint not available (404)")
				return
			}
			t.Errorf("failed to get compute resources: %v", err)
		}
		t.Logf("Found %d compute resources", len(resources))

		// Log resource details
		if len(resources) > 0 {
			for _, resource := range resources {
				t.Logf("Compute Resource: %s (%s) - Available: %v, CPU: %s, Memory: %s",
					resource.Name, resource.Type, resource.Available, resource.CPU, resource.Memory)
			}
		}
	})

	t.Run("get secrets", func(t *testing.T) {
		secrets, err := client.ContainerDeployments.GetSecrets(ctx)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Secrets endpoint not available (404)")
				return
			}
			t.Errorf("failed to get secrets: %v", err)
		}
		t.Logf("Found %d secrets", len(secrets))
	})
}
