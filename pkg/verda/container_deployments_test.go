// Copyright 2026 Verda Cloud Oy
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package verda

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestContainerDeploymentsService_GetDeployments(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

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
			if container.Image.Image == "" {
				t.Error("expected image to be set")
			}
		}
	})
}

func TestContainerDeploymentsService_CreateDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	t.Run("create deployment", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:   "llm-inference",
			IsSpot: false,
			ContainerRegistrySettings: ContainerRegistrySettings{
				IsPrivate: true,
				Credentials: &RegistryCredentialsRef{
					Name: "dockerhub-credentials",
				},
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "registry-1.docker.io/chentex/random-logger:v1.0.1",
					ExposedPort: 8080,
					Healthcheck: &ContainerHealthcheck{
						Enabled: true,
						Port:    8081,
						Path:    "/health",
					},
					EntrypointOverrides: &ContainerEntrypointOverrides{
						Enabled:    true,
						Entrypoint: []string{"python3", "main.py"},
						Cmd:        []string{"--port", "8080"},
					},
					Env: []ContainerEnvVar{
						{
							Name:                     "MY_ENV_VAR",
							ValueOrReferenceToSecret: "my-value",
							Type:                     "plain",
						},
					},
					VolumeMounts: []ContainerVolumeMount{
						{
							Type:      "scratch",
							MountPath: "/data",
						},
					},
				},
			},
			Compute: ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Scaling: ContainerScalingOptions{
				MinReplicaCount:              1,
				MaxReplicaCount:              1,
				ScaleDownPolicy:              &ScalingPolicy{DelaySeconds: 300},
				ScaleUpPolicy:                &ScalingPolicy{DelaySeconds: 300},
				QueueMessageTTLSeconds:       300,
				ConcurrentRequestsPerReplica: 1,
				ScalingTriggers: &ScalingTriggers{
					QueueLoad:      &QueueLoadTrigger{Threshold: 1}, // Must be >= 1
					CPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 80},
					GPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 80},
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

	t.Run("validation - missing compute", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateDeploymentRequest{
			Name:                      "test",
			ContainerRegistrySettings: ContainerRegistrySettings{IsPrivate: false},
			Compute:                   ContainerCompute{},
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
			ContainerRegistrySettings: ContainerRegistrySettings{IsPrivate: false},
			Compute:                   ContainerCompute{Name: "H100", Size: 1},
			Scaling:                   ContainerScalingOptions{},
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
			ContainerRegistrySettings: ContainerRegistrySettings{IsPrivate: false},
			Compute:                   ContainerCompute{Name: "H100", Size: 1},
			Scaling:                   ContainerScalingOptions{MaxReplicaCount: 1},
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
			ContainerRegistrySettings: ContainerRegistrySettings{IsPrivate: false},
			Compute:                   ContainerCompute{Name: "H100", Size: 1},
			Scaling:                   ContainerScalingOptions{MaxReplicaCount: 1},
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
			ContainerRegistrySettings: ContainerRegistrySettings{IsPrivate: false},
			Compute:                   ContainerCompute{Name: "H100", Size: 1},
			Scaling:                   ContainerScalingOptions{MaxReplicaCount: 1},
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

	client := NewTestClient(t, mockServer)

	t.Run("validation - nil request", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.ContainerDeployments.UpdateDeployment(ctx, "test-deployment", nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("validation - empty deployment name", func(t *testing.T) {
		ctx := context.Background()
		maxReplicas := 2
		req := &UpdateDeploymentRequest{
			Scaling: &ContainerScalingOptions{
				MaxReplicaCount: maxReplicas,
			},
		}
		_, err := client.ContainerDeployments.UpdateDeployment(ctx, "", req)
		if err == nil {
			t.Error("expected error for empty deployment name")
		}
	})

	t.Run("partial update - scaling only", func(t *testing.T) {
		ctx := context.Background()
		maxReplicas := 3
		req := &UpdateDeploymentRequest{
			Scaling: &ContainerScalingOptions{
				MaxReplicaCount: maxReplicas,
			},
		}
		// This should not fail validation - partial updates are allowed
		_, err := client.ContainerDeployments.UpdateDeployment(ctx, "test-deployment", req)
		// The mock server may return an error, but validation should pass
		if err != nil && err.Error() == "at least one container is required" {
			t.Error("UpdateDeployment should allow partial updates without containers")
		}
	})

	t.Run("container update - with name", func(t *testing.T) {
		ctx := context.Background()
		req := &UpdateDeploymentRequest{
			Containers: []CreateDeploymentContainer{
				{
					Name:        "my-container",
					Image:       "nginx:latest",
					ExposedPort: 8080,
				},
			},
		}
		deployment, err := client.ContainerDeployments.UpdateDeployment(ctx, "test-deployment", req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if deployment == nil {
			t.Fatal("expected deployment, got nil")
		}
	})

	t.Run("validation - updating container without name", func(t *testing.T) {
		ctx := context.Background()
		req := &UpdateDeploymentRequest{
			Containers: []CreateDeploymentContainer{
				{
					Image:       "nginx:latest",
					ExposedPort: 8080,
					// Name is missing - API requires it for updates
				},
			},
		}
		// Should fail - API requires container name for updates
		_, err := client.ContainerDeployments.UpdateDeployment(ctx, "test-deployment", req)
		if err == nil {
			t.Error("expected error when container name is missing")
		}
	})
}

func TestContainerDeploymentsService_GetServerlessComputeResources(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

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
			if resource.Size == 0 {
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
				if resource.Size == 0 {
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

	client := NewTestClient(t, mockServer)

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
			if secret.CreatedAt.IsZero() {
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
				if secret.CreatedAt.IsZero() {
					t.Errorf("secret %d missing CreatedAt", i)
				}
				if secret.SecretType == "" {
					t.Errorf("secret %d missing SecretType", i)
				}
			}
		}
	})
}

func TestValidateCreateDeploymentRequest(t *testing.T) {
	t.Run("valid request passes", func(t *testing.T) {
		req := &CreateDeploymentRequest{
			Name: "test-deployment",
			Compute: ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "nginx:1.25.3",
					ExposedPort: 80,
				},
			},
			Scaling: ContainerScalingOptions{
				MaxReplicaCount: 1,
				ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 300},
				ScaleUpPolicy:   &ScalingPolicy{DelaySeconds: 60},
				ScalingTriggers: &ScalingTriggers{
					QueueLoad: &QueueLoadTrigger{Threshold: 1},
				},
			},
		}
		err := ValidateCreateDeploymentRequest(req)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("latest tag rejected", func(t *testing.T) {
		req := &CreateDeploymentRequest{
			Name: "test-deployment",
			Compute: ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "nginx:latest",
					ExposedPort: 80,
				},
			},
			Scaling: ContainerScalingOptions{
				MaxReplicaCount: 1,
				ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 300},
				ScaleUpPolicy:   &ScalingPolicy{DelaySeconds: 60},
				ScalingTriggers: &ScalingTriggers{},
			},
		}
		err := ValidateCreateDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for latest tag, got nil")
		}
	})

	t.Run("no tag rejected (defaults to latest)", func(t *testing.T) {
		req := &CreateDeploymentRequest{
			Name: "test-deployment",
			Compute: ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "nginx",
					ExposedPort: 80,
				},
			},
			Scaling: ContainerScalingOptions{
				MaxReplicaCount: 1,
				ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 300},
				ScaleUpPolicy:   &ScalingPolicy{DelaySeconds: 60},
				ScalingTriggers: &ScalingTriggers{},
			},
		}
		err := ValidateCreateDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for image without tag, got nil")
		}
	})

	t.Run("missing scale_down_policy rejected", func(t *testing.T) {
		req := &CreateDeploymentRequest{
			Name: "test-deployment",
			Compute: ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "nginx:1.25.3",
					ExposedPort: 80,
				},
			},
			Scaling: ContainerScalingOptions{
				MaxReplicaCount: 1,
				ScaleUpPolicy:   &ScalingPolicy{DelaySeconds: 60},
				ScalingTriggers: &ScalingTriggers{},
			},
		}
		err := ValidateCreateDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for missing scale_down_policy, got nil")
		}
	})

	t.Run("queue_load threshold < 1 rejected", func(t *testing.T) {
		req := &CreateDeploymentRequest{
			Name: "test-deployment",
			Compute: ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "nginx:1.25.3",
					ExposedPort: 80,
				},
			},
			Scaling: ContainerScalingOptions{
				MaxReplicaCount: 1,
				ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 300},
				ScaleUpPolicy:   &ScalingPolicy{DelaySeconds: 60},
				ScalingTriggers: &ScalingTriggers{
					QueueLoad: &QueueLoadTrigger{Threshold: 0.5},
				},
			},
		}
		err := ValidateCreateDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for queue_load threshold < 1, got nil")
		}
	})

	t.Run("missing scale_up_policy rejected", func(t *testing.T) {
		req := &CreateDeploymentRequest{
			Name: "test-deployment",
			Compute: ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "nginx:1.25.3",
					ExposedPort: 80,
				},
			},
			Scaling: ContainerScalingOptions{
				MaxReplicaCount: 1,
				ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 300},
			},
		}
		err := ValidateCreateDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for missing scale_up_policy, got nil")
		}
	})
}

// --- IsInferno tests ---

func TestContainerDeploymentsService_CreateDeployment_Inferno(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	req := &CreateDeploymentRequest{
		Name:      "inferno-deployment",
		IsInferno: true,
		Compute:   ContainerCompute{Name: "H100", Size: 1},
		Containers: []CreateDeploymentContainer{
			{Image: "myimage:v1.0", ExposedPort: 8080},
		},
		Scaling: ContainerScalingOptions{MaxReplicaCount: 1},
	}

	deployment, err := client.ContainerDeployments.CreateDeployment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected deployment, got nil")
	}
}

func TestContainerDeploymentsService_UpdateDeployment_Inferno(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	infernoTrue := true
	req := &UpdateDeploymentRequest{
		IsInferno: &infernoTrue,
		Containers: []CreateDeploymentContainer{
			{Name: "my-container", Image: "nginx:1.25", ExposedPort: 80},
		},
	}

	deployment, err := client.ContainerDeployments.UpdateDeployment(ctx, "test-deployment", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected deployment, got nil")
	}
}

func TestContainerDeployment_IsInferno_JSONSerialization(t *testing.T) {
	t.Run("CreateDeploymentRequest with is_inferno true", func(t *testing.T) {
		req := CreateDeploymentRequest{
			Name:      "test",
			IsInferno: true,
			Compute:   ContainerCompute{Name: "H100", Size: 1},
			Containers: []CreateDeploymentContainer{
				{Image: "nginx:1.25", ExposedPort: 80},
			},
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if v, ok := m["is_inferno"].(bool); !ok || !v {
			t.Errorf("expected is_inferno=true in JSON, got %v", m["is_inferno"])
		}
	})

	t.Run("CreateDeploymentRequest with is_inferno false", func(t *testing.T) {
		req := CreateDeploymentRequest{
			Name:      "test",
			IsInferno: false,
			Compute:   ContainerCompute{Name: "H100", Size: 1},
			Containers: []CreateDeploymentContainer{
				{Image: "nginx:1.25", ExposedPort: 80},
			},
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if v, ok := m["is_inferno"].(bool); !ok || v {
			t.Errorf("expected is_inferno=false in JSON, got %v", m["is_inferno"])
		}
	})

	t.Run("UpdateDeploymentRequest with is_inferno pointer", func(t *testing.T) {
		infernoTrue := true
		req := UpdateDeploymentRequest{IsInferno: &infernoTrue}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if v, ok := m["is_inferno"].(bool); !ok || !v {
			t.Errorf("expected is_inferno=true in JSON, got %v", m["is_inferno"])
		}
	})

	t.Run("UpdateDeploymentRequest without is_inferno omits field", func(t *testing.T) {
		req := UpdateDeploymentRequest{}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if _, exists := m["is_inferno"]; exists {
			t.Error("expected is_inferno to be omitted when nil")
		}
	})
}

// --- GetDeploymentsForProject ---

func TestContainerDeploymentsService_GetDeploymentsForProject(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	t.Run("with project ID", func(t *testing.T) {
		ctx := context.Background()
		deployments, err := client.ContainerDeployments.GetDeploymentsForProject(ctx, "proj-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deployments) == 0 {
			t.Error("expected at least one deployment")
		}
	})

	t.Run("without project ID", func(t *testing.T) {
		ctx := context.Background()
		deployments, err := client.ContainerDeployments.GetDeploymentsForProject(ctx, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deployments) == 0 {
			t.Error("expected at least one deployment")
		}
	})
}

// --- GetDeploymentByName ---

func TestContainerDeploymentsService_GetDeploymentByName(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	deployment, err := client.ContainerDeployments.GetDeploymentByName(ctx, "test-deployment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected deployment, got nil")
	}
	if deployment.Name != "test-deployment" {
		t.Errorf("expected name 'test-deployment', got '%s'", deployment.Name)
	}
}

// --- DeleteDeployment ---

func TestContainerDeploymentsService_DeleteDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("empty name error", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteDeployment(ctx, "", 0)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("timeout 0 - skip waiting", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteDeployment(ctx, "my-deployment", 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("timeout negative - use API default", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteDeployment(ctx, "my-deployment", -1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("timeout 500ms", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteDeployment(ctx, "my-deployment", 500)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("timeout capped at 300000", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteDeployment(ctx, "my-deployment", 400000)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- GetDeploymentStatus ---

func TestContainerDeploymentsService_GetDeploymentStatus(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	status, err := client.ContainerDeployments.GetDeploymentStatus(ctx, "test-deployment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status == nil {
		t.Fatal("expected status, got nil")
	}
	if status.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", status.Status)
	}
}

// --- RestartDeployment ---

func TestContainerDeploymentsService_RestartDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	err := client.ContainerDeployments.RestartDeployment(context.Background(), "test-deployment")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- PauseDeployment ---

func TestContainerDeploymentsService_PauseDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	err := client.ContainerDeployments.PauseDeployment(context.Background(), "test-deployment")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- ResumeDeployment ---

func TestContainerDeploymentsService_ResumeDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	err := client.ContainerDeployments.ResumeDeployment(context.Background(), "test-deployment")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- PurgeDeploymentQueue ---

func TestContainerDeploymentsService_PurgeDeploymentQueue(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	err := client.ContainerDeployments.PurgeDeploymentQueue(context.Background(), "test-deployment")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- GetDeploymentScaling ---

func TestContainerDeploymentsService_GetDeploymentScaling(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		scaling, err := client.ContainerDeployments.GetDeploymentScaling(ctx, "test-deployment")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scaling == nil {
			t.Fatal("expected scaling, got nil")
		}
		if scaling.MaxReplicaCount != 50 {
			t.Errorf("expected max_replica_count 50, got %d", scaling.MaxReplicaCount)
		}
	})

	t.Run("empty name error", func(t *testing.T) {
		_, err := client.ContainerDeployments.GetDeploymentScaling(ctx, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

// --- UpdateDeploymentScaling ---

func TestContainerDeploymentsService_UpdateDeploymentScaling(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		maxReplicas := 10
		req := &UpdateScalingOptionsRequest{
			MaxReplicaCount: &maxReplicas,
		}
		scaling, err := client.ContainerDeployments.UpdateDeploymentScaling(ctx, "test-deployment", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scaling == nil {
			t.Fatal("expected scaling, got nil")
		}
	})

	t.Run("empty name error", func(t *testing.T) {
		maxReplicas := 10
		req := &UpdateScalingOptionsRequest{MaxReplicaCount: &maxReplicas}
		_, err := client.ContainerDeployments.UpdateDeploymentScaling(ctx, "", req)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("nil request error", func(t *testing.T) {
		_, err := client.ContainerDeployments.UpdateDeploymentScaling(ctx, "test-deployment", nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})
}

// --- GetDeploymentReplicas ---

func TestContainerDeploymentsService_GetDeploymentReplicas(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	replicas, err := client.ContainerDeployments.GetDeploymentReplicas(ctx, "test-deployment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if replicas == nil {
		t.Fatal("expected replicas, got nil")
	}
	if len(replicas.List) == 0 {
		t.Error("expected at least one replica")
	}
	if replicas.List[0].ID != "replica-1" {
		t.Errorf("expected replica ID 'replica-1', got '%s'", replicas.List[0].ID)
	}
	if replicas.List[0].Status != "running" {
		t.Errorf("expected replica status 'running', got '%s'", replicas.List[0].Status)
	}
}

// --- GetEnvironmentVariables ---

func TestContainerDeploymentsService_GetEnvironmentVariables(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		envVars, err := client.ContainerDeployments.GetEnvironmentVariables(ctx, "test-deployment")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(envVars) == 0 {
			t.Error("expected at least one env var")
		}
		if envVars[0].Name != "MY_VAR" {
			t.Errorf("expected env var name 'MY_VAR', got '%s'", envVars[0].Name)
		}
	})

	t.Run("empty name error", func(t *testing.T) {
		_, err := client.ContainerDeployments.GetEnvironmentVariables(ctx, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

// --- AddEnvironmentVariables ---

func TestContainerDeploymentsService_AddEnvironmentVariables(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &ContainerEnvVarsRequest{
			ContainerName: "my-container",
			Env: []ContainerEnvVar{
				{Name: "KEY", ValueOrReferenceToSecret: "val", Type: "plain"},
			},
		}
		err := client.ContainerDeployments.AddEnvironmentVariables(ctx, "test-deployment", req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty deployment name", func(t *testing.T) {
		req := &ContainerEnvVarsRequest{
			ContainerName: "c", Env: []ContainerEnvVar{{Name: "K", ValueOrReferenceToSecret: "v", Type: "plain"}},
		}
		err := client.ContainerDeployments.AddEnvironmentVariables(ctx, "", req)
		if err == nil {
			t.Error("expected error for empty deployment name")
		}
	})

	t.Run("nil request", func(t *testing.T) {
		err := client.ContainerDeployments.AddEnvironmentVariables(ctx, "test-deployment", nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("empty container name", func(t *testing.T) {
		req := &ContainerEnvVarsRequest{
			ContainerName: "",
			Env:           []ContainerEnvVar{{Name: "K", ValueOrReferenceToSecret: "v", Type: "plain"}},
		}
		err := client.ContainerDeployments.AddEnvironmentVariables(ctx, "test-deployment", req)
		if err == nil {
			t.Error("expected error for empty container name")
		}
	})

	t.Run("empty env array", func(t *testing.T) {
		req := &ContainerEnvVarsRequest{
			ContainerName: "my-container",
			Env:           []ContainerEnvVar{},
		}
		err := client.ContainerDeployments.AddEnvironmentVariables(ctx, "test-deployment", req)
		if err == nil {
			t.Error("expected error for empty env array")
		}
	})
}

// --- UpdateEnvironmentVariables ---

func TestContainerDeploymentsService_UpdateEnvironmentVariables(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &ContainerEnvVarsRequest{
			ContainerName: "my-container",
			Env: []ContainerEnvVar{
				{Name: "KEY", ValueOrReferenceToSecret: "new-val", Type: "plain"},
			},
		}
		err := client.ContainerDeployments.UpdateEnvironmentVariables(ctx, "test-deployment", req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty deployment name", func(t *testing.T) {
		req := &ContainerEnvVarsRequest{
			ContainerName: "c", Env: []ContainerEnvVar{{Name: "K", ValueOrReferenceToSecret: "v", Type: "plain"}},
		}
		err := client.ContainerDeployments.UpdateEnvironmentVariables(ctx, "", req)
		if err == nil {
			t.Error("expected error for empty deployment name")
		}
	})

	t.Run("nil request", func(t *testing.T) {
		err := client.ContainerDeployments.UpdateEnvironmentVariables(ctx, "test-deployment", nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("empty container name", func(t *testing.T) {
		req := &ContainerEnvVarsRequest{
			ContainerName: "",
			Env:           []ContainerEnvVar{{Name: "K", ValueOrReferenceToSecret: "v", Type: "plain"}},
		}
		err := client.ContainerDeployments.UpdateEnvironmentVariables(ctx, "test-deployment", req)
		if err == nil {
			t.Error("expected error for empty container name")
		}
	})

	t.Run("empty env array", func(t *testing.T) {
		req := &ContainerEnvVarsRequest{
			ContainerName: "my-container",
			Env:           []ContainerEnvVar{},
		}
		err := client.ContainerDeployments.UpdateEnvironmentVariables(ctx, "test-deployment", req)
		if err == nil {
			t.Error("expected error for empty env array")
		}
	})
}

// --- DeleteEnvironmentVariables ---

func TestContainerDeploymentsService_DeleteEnvironmentVariables(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &DeleteContainerEnvVarsRequest{
			ContainerName: "my-container",
			Env:           []string{"KEY"},
		}
		err := client.ContainerDeployments.DeleteEnvironmentVariables(ctx, "test-deployment", req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty deployment name", func(t *testing.T) {
		req := &DeleteContainerEnvVarsRequest{ContainerName: "c", Env: []string{"K"}}
		err := client.ContainerDeployments.DeleteEnvironmentVariables(ctx, "", req)
		if err == nil {
			t.Error("expected error for empty deployment name")
		}
	})

	t.Run("nil request", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteEnvironmentVariables(ctx, "test-deployment", nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("empty container name", func(t *testing.T) {
		req := &DeleteContainerEnvVarsRequest{ContainerName: "", Env: []string{"K"}}
		err := client.ContainerDeployments.DeleteEnvironmentVariables(ctx, "test-deployment", req)
		if err == nil {
			t.Error("expected error for empty container name")
		}
	})

	t.Run("empty env array", func(t *testing.T) {
		req := &DeleteContainerEnvVarsRequest{ContainerName: "my-container", Env: []string{}}
		err := client.ContainerDeployments.DeleteEnvironmentVariables(ctx, "test-deployment", req)
		if err == nil {
			t.Error("expected error for empty env array")
		}
	})
}

// --- CreateSecret ---

func TestContainerDeploymentsService_CreateSecret(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &CreateSecretRequest{Name: "my-secret", Value: "my-value"}
		err := client.ContainerDeployments.CreateSecret(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("nil request", func(t *testing.T) {
		err := client.ContainerDeployments.CreateSecret(ctx, nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		req := &CreateSecretRequest{Name: "", Value: "val"}
		err := client.ContainerDeployments.CreateSecret(ctx, req)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("empty value", func(t *testing.T) {
		req := &CreateSecretRequest{Name: "name", Value: ""}
		err := client.ContainerDeployments.CreateSecret(ctx, req)
		if err == nil {
			t.Error("expected error for empty value")
		}
	})
}

// --- DeleteSecret ---

func TestContainerDeploymentsService_DeleteSecret(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("without force", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteSecret(ctx, "my-secret", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with force", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteSecret(ctx, "my-secret", true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- GetFileSecrets ---

func TestContainerDeploymentsService_GetFileSecrets(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	secrets, err := client.ContainerDeployments.GetFileSecrets(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secrets) == 0 {
		t.Error("expected at least one file secret")
	}
	if secrets[0].Name != "test-file-secret" {
		t.Errorf("expected name 'test-file-secret', got '%s'", secrets[0].Name)
	}
	if len(secrets[0].FileNames) == 0 {
		t.Error("expected file names to be populated")
	}
}

// --- CreateFileSecret ---

func TestContainerDeploymentsService_CreateFileSecret(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &CreateFileSecretRequest{
			Name: "my-file-secret",
			Files: []FileSecretFile{
				{Name: "config.json", Base64Content: "eyJ0ZXN0IjogdHJ1ZX0="},
			},
		}
		err := client.ContainerDeployments.CreateFileSecret(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("nil request", func(t *testing.T) {
		err := client.ContainerDeployments.CreateFileSecret(ctx, nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		req := &CreateFileSecretRequest{
			Name:  "",
			Files: []FileSecretFile{{Name: "f.txt", Base64Content: "YQ=="}},
		}
		err := client.ContainerDeployments.CreateFileSecret(ctx, req)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("no files", func(t *testing.T) {
		req := &CreateFileSecretRequest{
			Name:  "my-secret",
			Files: []FileSecretFile{},
		}
		err := client.ContainerDeployments.CreateFileSecret(ctx, req)
		if err == nil {
			t.Error("expected error for empty files")
		}
	})
}

// --- DeleteFileSecret ---

func TestContainerDeploymentsService_DeleteFileSecret(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("without force", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteFileSecret(ctx, "my-file-secret", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with force", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteFileSecret(ctx, "my-file-secret", true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- GetRegistryCredentials ---

func TestContainerDeploymentsService_GetRegistryCredentials(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	creds, err := client.ContainerDeployments.GetRegistryCredentials(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(creds) == 0 {
		t.Error("expected at least one credential")
	}
	if creds[0].Name != "dockerhub-credentials" {
		t.Errorf("expected name 'dockerhub-credentials', got '%s'", creds[0].Name)
	}
}

// --- CreateRegistryCredentials ---

func TestContainerDeploymentsService_CreateRegistryCredentials(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &CreateRegistryCredentialsRequest{
			Name:     "my-creds",
			Type:     "dockerhub",
			Username: "user",
		}
		err := client.ContainerDeployments.CreateRegistryCredentials(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("nil request", func(t *testing.T) {
		err := client.ContainerDeployments.CreateRegistryCredentials(ctx, nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		req := &CreateRegistryCredentialsRequest{Name: "", Type: "dockerhub"}
		err := client.ContainerDeployments.CreateRegistryCredentials(ctx, req)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("empty type", func(t *testing.T) {
		req := &CreateRegistryCredentialsRequest{Name: "creds", Type: ""}
		err := client.ContainerDeployments.CreateRegistryCredentials(ctx, req)
		if err == nil {
			t.Error("expected error for empty type")
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		req := &CreateRegistryCredentialsRequest{Name: "creds", Type: "invalid-type"}
		err := client.ContainerDeployments.CreateRegistryCredentials(ctx, req)
		if err == nil {
			t.Error("expected error for invalid type")
		}
	})
}

// --- DeleteRegistryCredentials ---

func TestContainerDeploymentsService_DeleteRegistryCredentials(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("without force", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteRegistryCredentials(ctx, "my-creds", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with force", func(t *testing.T) {
		err := client.ContainerDeployments.DeleteRegistryCredentials(ctx, "my-creds", true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- Standalone Validate() tests ---

func TestCreateSecretRequest_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		req := CreateSecretRequest{Name: "s", Value: "v"}
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		req := CreateSecretRequest{Name: "", Value: "v"}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("empty value", func(t *testing.T) {
		req := CreateSecretRequest{Name: "s", Value: ""}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty value")
		}
	})
}

func TestCreateFileSecretRequest_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		req := CreateFileSecretRequest{
			Name:  "s",
			Files: []FileSecretFile{{Name: "f.txt", Base64Content: "YQ=="}},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		req := CreateFileSecretRequest{
			Name:  "",
			Files: []FileSecretFile{{Name: "f.txt", Base64Content: "YQ=="}},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("no files", func(t *testing.T) {
		req := CreateFileSecretRequest{Name: "s", Files: []FileSecretFile{}}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty files")
		}
	})
}

func TestCreateRegistryCredentialsRequest_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		req := CreateRegistryCredentialsRequest{Name: "c", Type: "dockerhub"}
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		req := CreateRegistryCredentialsRequest{Name: "", Type: "dockerhub"}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("empty type", func(t *testing.T) {
		req := CreateRegistryCredentialsRequest{Name: "c", Type: ""}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty type")
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		req := CreateRegistryCredentialsRequest{Name: "c", Type: "azure"}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid type")
		}
	})

	t.Run("all valid types", func(t *testing.T) {
		validTypes := []string{"verda", "gcr", "dockerhub", "ghcr", "aws-ecr", "scaleway", "custom"}
		for _, vt := range validTypes {
			req := CreateRegistryCredentialsRequest{Name: "c", Type: vt}
			if err := req.Validate(); err != nil {
				t.Errorf("expected type %q to be valid, got error: %v", vt, err)
			}
		}
	})
}
