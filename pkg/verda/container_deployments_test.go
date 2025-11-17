package verda

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestContainerDeploymentsService_GetDeployments(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get all deployments", func(t *testing.T) {
		ctx := context.Background()
		deployments, err := client.ContainerDeployments.GetDeployments(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(deployments) == 0 {
			t.Error("expected at least one deployment")
		}

		// Verify first deployment has expected fields
		if len(deployments) > 0 {
			deployment := deployments[0]
			if deployment.Name == "" {
				t.Error("expected deployment to have a Name")
			}
			if deployment.Image == "" {
				t.Error("expected deployment to have an Image")
			}
			if deployment.Status == "" {
				t.Error("expected deployment to have a Status")
			}
		}
	})
}

func TestContainerDeploymentsService_CreateDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("create deployment", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:     "test-deployment",
			Image:    "nginx:latest",
			Replicas: 2,
		}

		deployment, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if deployment == nil {
			t.Fatal("expected deployment, got nil")
		}

		if deployment.Name == "" {
			t.Error("expected deployment to have a Name")
		}
		if deployment.Image == "" {
			t.Error("expected deployment to have an Image")
		}
	})
}

func TestContainerDeploymentsService_GetServerlessComputeResources(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get compute resources", func(t *testing.T) {
		ctx := context.Background()
		resources, err := client.ContainerDeployments.GetServerlessComputeResources(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(resources) == 0 {
			t.Error("expected at least one compute resource")
		}

		// Verify first resource has expected fields
		if len(resources) > 0 {
			resource := resources[0]
			if resource.Name == "" {
				t.Error("expected resource to have a Name")
			}
			if resource.Type == "" {
				t.Error("expected resource to have a Type")
			}
		}
	})

	t.Run("verify resource structure", func(t *testing.T) {
		ctx := context.Background()
		resources, err := client.ContainerDeployments.GetServerlessComputeResources(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(resources) > 0 {
			for i, resource := range resources {
				if resource.Name == "" {
					t.Errorf("resource %d missing Name", i)
				}
				if resource.Type == "" {
					t.Errorf("resource %d missing Type", i)
				}
				// Available is a boolean, always has a value
			}
		}
	})

	t.Run("verify at least one available resource", func(t *testing.T) {
		ctx := context.Background()
		resources, err := client.ContainerDeployments.GetServerlessComputeResources(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		hasAvailableResource := false
		for _, resource := range resources {
			if resource.Available {
				hasAvailableResource = true
				break
			}
		}

		if !hasAvailableResource {
			t.Error("expected at least one available compute resource")
		}
	})
}

func TestContainerDeploymentsService_GetSecrets(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get secrets", func(t *testing.T) {
		ctx := context.Background()
		secrets, err := client.ContainerDeployments.GetSecrets(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(secrets) == 0 {
			t.Error("expected at least one secret")
		}

		// Verify first secret has expected fields
		if len(secrets) > 0 {
			secret := secrets[0]
			if secret.Name == "" {
				t.Error("expected secret to have a Name")
			}
			if secret.CreatedAt == "" {
				t.Error("expected secret to have a CreatedAt")
			}
		}
	})

	t.Run("verify secrets structure", func(t *testing.T) {
		ctx := context.Background()
		secrets, err := client.ContainerDeployments.GetSecrets(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(secrets) > 0 {
			for i, secret := range secrets {
				if secret.Name == "" {
					t.Errorf("secret %d missing Name", i)
				}
				if secret.CreatedAt == "" {
					t.Errorf("secret %d missing CreatedAt", i)
				}
			}
		}
	})
}
