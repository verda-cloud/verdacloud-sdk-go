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
			if deployment.Name != "flux" {
				t.Errorf("expected deployment name 'flux', got '%s'", deployment.Name)
			}
			if len(deployment.Containers) == 0 {
				t.Error("expected deployment to have Containers")
			}
			if deployment.EndpointBaseURL != "https://containers.datacrunch.io/flux" {
				t.Errorf("expected endpoint URL 'https://containers.datacrunch.io/flux', got '%s'", deployment.EndpointBaseURL)
			}
			if deployment.Compute == nil || deployment.Compute.Name != "H100" {
				t.Error("expected compute to be H100")
			}

			// Verify container structure
			container := deployment.Containers[0]
			if container.Name != "random-logger-0" {
				t.Errorf("expected container name 'random-logger-0', got '%s'", container.Name)
			}
			if container.ExposedPort != 8080 {
				t.Errorf("expected exposed port 8080, got %d", container.ExposedPort)
			}

			// Verify flexible fields
			if container.Healthcheck == nil {
				t.Error("expected healthcheck to be set")
			}
			if container.EntrypointOverrides == nil {
				t.Error("expected entrypoint_overrides to be set")
			}
			if len(container.VolumeMounts) == 0 {
				t.Error("expected volume_mounts to be set")
			}
			if container.Image == nil {
				t.Error("expected image to be set")
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
			Name:   "llm-inference",
			IsSpot: false,
			ContainerRegistrySettings: map[string]any{
				"is_private": true,
				"credentials": map[string]any{
					"name": "dockerhub-credentials",
				},
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "registry-1.docker.io/chentex/random-logger:v1.0.1",
					ExposedPort: 8080,
					Healthcheck: map[string]any{
						"enabled": true,
						"port":    8081,
						"path":    "/health",
					},
					EntrypointOverrides: map[string]any{
						"enabled":    true,
						"entrypoint": []string{"python3", "main.py"},
						"cmd":        []string{"--port", "8080"},
					},
					Env: []ContainerEnvVar{
						{
							Name:                     "MY_ENV_VAR",
							ValueOrReferenceToSecret: "my-value",
							Type:                     "plain",
						},
					},
					VolumeMounts: []map[string]any{
						{
							"type":       "scratch",
							"mount_path": "/data",
						},
					},
				},
			},
			Compute: map[string]any{
				"name": "H100",
				"size": 1,
			},
			Scaling: map[string]any{
				"min_replica_count": 1,
				"max_replica_count": 1,
				"scale_down_policy": map[string]any{
					"delay_seconds": 300,
				},
				"scale_up_policy": map[string]any{
					"delay_seconds": 300,
				},
				"queue_message_ttl_seconds":       300,
				"concurrent_requests_per_replica": 1,
				"scaling_triggers": map[string]any{
					"queue_load": map[string]any{
						"threshold": 0.5,
					},
					"cpu_utilization": map[string]any{
						"enabled":   true,
						"threshold": 80,
					},
					"gpu_utilization": map[string]any{
						"enabled":   true,
						"threshold": 80,
					},
				},
			},
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
		if len(deployment.Containers) == 0 {
			t.Error("expected deployment to have Containers")
		}
	})

	t.Run("validation - nil request", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.ContainerDeployments.CreateDeployment(ctx, nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("validation - empty name", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name: "",
		}
		_, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("validation - missing container_registry_settings", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:                      "test",
			ContainerRegistrySettings: nil,
		}
		_, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for missing container_registry_settings")
		}
	})

	t.Run("validation - missing compute", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:                      "test",
			ContainerRegistrySettings: map[string]any{"is_private": false},
			Compute:                   nil,
		}
		_, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for missing compute")
		}
	})

	t.Run("validation - missing scaling", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:                      "test",
			ContainerRegistrySettings: map[string]any{"is_private": false},
			Compute:                   map[string]any{"name": "H100", "size": 1},
			Scaling:                   nil,
		}
		_, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for missing scaling")
		}
	})

	t.Run("validation - no containers", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:                      "test",
			ContainerRegistrySettings: map[string]any{"is_private": false},
			Compute:                   map[string]any{"name": "H100", "size": 1},
			Scaling:                   map[string]any{"max_replica_count": 1},
			Containers:                []CreateDeploymentContainer{},
		}
		_, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for empty containers")
		}
	})

	t.Run("validation - container without image", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:                      "test",
			ContainerRegistrySettings: map[string]any{"is_private": false},
			Compute:                   map[string]any{"name": "H100", "size": 1},
			Scaling:                   map[string]any{"max_replica_count": 1},
			Containers:                []CreateDeploymentContainer{{Image: "", ExposedPort: 8080}},
		}
		_, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for container without image")
		}
	})

	t.Run("validation - container without exposed_port", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:                      "test",
			ContainerRegistrySettings: map[string]any{"is_private": false},
			Compute:                   map[string]any{"name": "H100", "size": 1},
			Scaling:                   map[string]any{"max_replica_count": 1},
			Containers:                []CreateDeploymentContainer{{Image: "nginx:latest", ExposedPort: 0}},
		}
		_, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for container without exposed_port")
		}
	})
}

func TestContainerDeploymentsService_UpdateDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("validation - nil request", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.ContainerDeployments.UpdateDeployment(ctx, "test-deployment", nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("validation - empty deployment name", func(t *testing.T) {
		ctx := context.Background()
		req := &UpdateDeploymentRequest{
			Scaling: map[string]any{
				"max_replica_count": 2,
			},
		}
		_, err := client.ContainerDeployments.UpdateDeployment(ctx, "", req)
		if err == nil {
			t.Error("expected error for empty deployment name")
		}
	})

	t.Run("partial update - scaling only", func(t *testing.T) {
		ctx := context.Background()
		req := &UpdateDeploymentRequest{
			Scaling: map[string]any{
				"max_replica_count": 3,
			},
		}
		// This should not fail validation - partial updates are allowed
		_, err := client.ContainerDeployments.UpdateDeployment(ctx, "test-deployment", req)
		// The mock server may return an error, but validation should pass
		if err != nil && err.Error() == "at least one container is required" {
			t.Error("UpdateDeployment should allow partial updates without containers")
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
			if resource.Size == "" {
				t.Error("expected resource to have a Size")
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
				if resource.Size == "" {
					t.Errorf("resource %d missing Size", i)
				}
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
			if resource.IsAvailable {
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
			if secret.SecretType == "" {
				t.Error("expected secret to have a SecretType")
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
				if secret.SecretType == "" {
					t.Errorf("secret %d missing SecretType", i)
				}
			}
		}
	})
}
