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
	"io"
	"net/http"
	"strings"
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
					QueueLoad:      &QueueLoadTrigger{Threshold: 1}, // 1..1000
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

	t.Run("queue_load threshold > 1000 rejected", func(t *testing.T) {
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
					QueueLoad: &QueueLoadTrigger{Threshold: 1500},
				},
			},
		}
		err := ValidateCreateDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for queue_load threshold > 1000, got nil")
		}
	})

	t.Run("scaling over API limits rejected", func(t *testing.T) {
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
				MaxReplicaCount: 1001,
				ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 300},
				ScaleUpPolicy:   &ScalingPolicy{DelaySeconds: 60},
			},
		}
		err := ValidateCreateDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for max_replica_count > 1000, got nil")
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

	var receivedInferno bool
	mockServer.SetHandler(http.MethodPost, "/container-deployments", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if v, ok := body["is_inferno"].(bool); ok {
			receivedInferno = v
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"name": "inferno-deployment",
			"containers": [{"name": "c-0", "image": {"image": "myimage:v1.0"}, "exposed_port": 8080}],
			"endpoint_base_url": "https://containers.datacrunch.io/inferno-deployment",
			"created_at": "2024-01-01T00:00:00.000Z",
			"compute": {"name": "H100", "size": 1},
			"is_spot": false
		}`))
	})

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
	if !receivedInferno {
		t.Error("server did not receive is_inferno=true in request body")
	}
}

func TestContainerDeploymentsService_UpdateDeployment_Inferno(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	var receivedInferno *bool
	mockServer.SetHandler(http.MethodPatch, "/container-deployments/test-deployment", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if v, ok := body["is_inferno"].(bool); ok {
			receivedInferno = &v
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "test-deployment",
			"containers": [{"name": "my-container", "image": {"image": "nginx:1.25"}, "exposed_port": 80}],
			"endpoint_base_url": "https://containers.datacrunch.io/test-deployment",
			"created_at": "2024-01-01T00:00:00.000Z",
			"compute": {"name": "H100", "size": 1},
			"is_spot": false
		}`))
	})

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
	if receivedInferno == nil || !*receivedInferno {
		t.Error("server did not receive is_inferno=true in PATCH request body")
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

		var m map[string]any
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

		var m map[string]any
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

		var m map[string]any
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

		var m map[string]any
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
	t.Run("empty name error", func(t *testing.T) {
		mockServer := testutil.NewMockServer()
		defer mockServer.Close()
		client := NewTestClient(t, mockServer)

		err := client.ContainerDeployments.DeleteDeployment(context.Background(), "", 0)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	tests := []struct {
		name      string
		timeoutMs int
		wantQuery string
	}{
		{"timeout 0 skips waiting", 0, "timeout=0"},
		{"negative timeout omits the parameter", -1, ""},
		{"timeout 500ms", 500, "timeout=500"},
		{"timeout at cap boundary", 300000, "timeout=300000"},
		{"timeout capped at 300000", 400000, "timeout=300000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := testutil.NewMockServer()
			defer mockServer.Close()

			var gotQuery string
			mockServer.SetHandler(http.MethodDelete, "/container-deployments/my-deployment", func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusNoContent)
			})

			client := NewTestClient(t, mockServer)

			err := client.ContainerDeployments.DeleteDeployment(context.Background(), "my-deployment", tt.timeoutMs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotQuery != tt.wantQuery {
				t.Errorf("query = %q, want %q", gotQuery, tt.wantQuery)
			}
		})
	}
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
	if status.Status != ContainerDeploymentStatusHealthy {
		t.Errorf("expected status %q, got %q", ContainerDeploymentStatusHealthy, status.Status)
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
	got := replicas.List[0]
	if got.Status != ReplicaStatusRunning {
		t.Errorf("expected replica status %q, got %q", ReplicaStatusRunning, got.Status)
	}
	if got.Image != "docker.io/library/nginx:1.21" || got.ImageName != "nginx" || got.ImageTag != "1.21" {
		t.Errorf("image fields = (%q, %q, %q), want (docker.io/library/nginx:1.21, nginx, 1.21)", got.Image, got.ImageName, got.ImageTag)
	}
}

// --- UpdateDeploymentScaling request body and validation ---

func TestContainerDeploymentsService_UpdateDeploymentScaling_RequestBody(t *testing.T) {
	tests := []struct {
		name string
		req  *UpdateScalingOptionsRequest
		want string
	}{
		{
			name: "scale to zero sends both counts",
			req:  &UpdateScalingOptionsRequest{MinReplicaCount: new(0), MaxReplicaCount: new(0)},
			want: `{"min_replica_count":0,"max_replica_count":0}`,
		},
		{
			name: "max only leaves other options out",
			req:  &UpdateScalingOptionsRequest{MaxReplicaCount: new(3)},
			want: `{"max_replica_count":3}`,
		},
		{
			name: "cpu trigger only leaves other triggers out",
			req: &UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{
				CPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 80},
			}},
			want: `{"scaling_triggers":{"cpu_utilization":{"enabled":true,"threshold":80}}}`,
		},
		{
			name: "disabling a trigger still sends it",
			req: &UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{
				GPUUtilization: &UtilizationTrigger{Enabled: false},
			}},
			want: `{"scaling_triggers":{"gpu_utilization":{"enabled":false,"threshold":0}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := testutil.NewMockServer()
			defer mockServer.Close()

			var got []byte
			mockServer.SetHandler(http.MethodPatch, "/container-deployments/my-deployment/scaling", func(w http.ResponseWriter, r *http.Request) {
				got, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			})
			client := NewTestClient(t, mockServer)

			if _, err := client.ContainerDeployments.UpdateDeploymentScaling(context.Background(), "my-deployment", tt.req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if body := strings.TrimSpace(string(got)); body != tt.want {
				t.Errorf("request body = %s, want %s", body, tt.want)
			}
		})
	}
}

func TestContainerDeploymentsService_UpdateDeploymentScaling_InvalidRequestNotSent(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	mockServer.SetHandler(http.MethodPatch, "/container-deployments/my-deployment/scaling", func(w http.ResponseWriter, _ *http.Request) {
		t.Error("invalid request reached the API")
		w.WriteHeader(http.StatusOK)
	})
	client := NewTestClient(t, mockServer)

	req := &UpdateScalingOptionsRequest{MinReplicaCount: new(5), MaxReplicaCount: new(1)}
	if _, err := client.ContainerDeployments.UpdateDeploymentScaling(context.Background(), "my-deployment", req); err == nil {
		t.Error("expected validation error for min_replica_count > max_replica_count")
	}
}

func TestUpdateScalingOptionsRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     UpdateScalingOptionsRequest
		wantErr bool
	}{
		{"empty request", UpdateScalingOptionsRequest{}, true},
		{"scale to zero", UpdateScalingOptionsRequest{MinReplicaCount: new(0), MaxReplicaCount: new(0)}, false},
		{"min equals max", UpdateScalingOptionsRequest{MinReplicaCount: new(2), MaxReplicaCount: new(2)}, false},
		{"min greater than max", UpdateScalingOptionsRequest{MinReplicaCount: new(5), MaxReplicaCount: new(1)}, true},
		{"only min set is not compared", UpdateScalingOptionsRequest{MinReplicaCount: new(5)}, false},
		{"negative min", UpdateScalingOptionsRequest{MinReplicaCount: new(-1)}, true},
		{"negative max", UpdateScalingOptionsRequest{MaxReplicaCount: new(-1)}, true},
		{"negative queue ttl", UpdateScalingOptionsRequest{QueueMessageTTLSeconds: new(-1)}, true},
		{"negative concurrent requests", UpdateScalingOptionsRequest{ConcurrentRequestsPerReplica: new(-1)}, true},
		{"negative scale down delay", UpdateScalingOptionsRequest{ScaleDownPolicy: &ScalingPolicy{DelaySeconds: -1}}, true},
		{"zero scale up delay", UpdateScalingOptionsRequest{ScaleUpPolicy: &ScalingPolicy{DelaySeconds: 0}}, false},
		{"scale up delay at max", UpdateScalingOptionsRequest{ScaleUpPolicy: &ScalingPolicy{DelaySeconds: 3600}}, false},
		{"scale down delay over max", UpdateScalingOptionsRequest{ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 3601}}, true},
		{"min replicas at max", UpdateScalingOptionsRequest{MinReplicaCount: new(1000)}, false},
		{"min replicas over max", UpdateScalingOptionsRequest{MinReplicaCount: new(1001)}, true},
		{"max replicas at max", UpdateScalingOptionsRequest{MaxReplicaCount: new(1000)}, false},
		{"max replicas over max", UpdateScalingOptionsRequest{MaxReplicaCount: new(1001)}, true},
		{"queue ttl at max", UpdateScalingOptionsRequest{QueueMessageTTLSeconds: new(86400)}, false},
		{"queue ttl over max", UpdateScalingOptionsRequest{QueueMessageTTLSeconds: new(86401)}, true},
		{"concurrent requests at max", UpdateScalingOptionsRequest{ConcurrentRequestsPerReplica: new(5000)}, false},
		{"concurrent requests over max", UpdateScalingOptionsRequest{ConcurrentRequestsPerReplica: new(5001)}, true},
		{"negative queue load threshold", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{Threshold: -0.5}}}, true},
		{"zero queue load threshold", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{Threshold: 0}}}, true},
		{"queue load threshold below 1", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{Threshold: 0.5}}}, true},
		{"queue load threshold at 1", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{Threshold: 1}}}, false},
		{"fractional queue load threshold", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{Threshold: 1.5}}}, false},
		{"queue load threshold at 1000", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{Threshold: 1000}}}, false},
		{"queue load threshold over 1000", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{Threshold: 1000.5}}}, true},
		{"cpu threshold below 5", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{CPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 4}}}, true},
		{"cpu threshold at 5", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{CPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 5}}}, false},
		{"cpu threshold at 100", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{CPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 100}}}, false},
		{"cpu threshold over 100", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{CPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 101}}}, true},
		{"gpu threshold below 5", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{GPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 4}}}, true},
		{"gpu threshold at 100", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{GPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 100}}}, false},
		{"enabled trigger with unset threshold", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{CPUUtilization: &UtilizationTrigger{Enabled: true}}}, false},
		{"disabled trigger threshold not checked", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{GPUUtilization: &UtilizationTrigger{Enabled: false, Threshold: 4}}}, false},
		{"empty triggers object", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainerScalingOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    ContainerScalingOptions
		wantErr bool
	}{
		{"zero values", ContainerScalingOptions{}, false},
		{"replicas at max", ContainerScalingOptions{MinReplicaCount: 1000, MaxReplicaCount: 1000}, false},
		{"min replicas over max", ContainerScalingOptions{MinReplicaCount: 1001}, true},
		{"max replicas over max", ContainerScalingOptions{MaxReplicaCount: 1001}, true},
		{"queue ttl at max", ContainerScalingOptions{QueueMessageTTLSeconds: 86400}, false},
		{"queue ttl over max", ContainerScalingOptions{QueueMessageTTLSeconds: 86401}, true},
		{"concurrent requests at max", ContainerScalingOptions{ConcurrentRequestsPerReplica: 5000}, false},
		{"concurrent requests over max", ContainerScalingOptions{ConcurrentRequestsPerReplica: 5001}, true},
		{"delay at max", ContainerScalingOptions{ScaleUpPolicy: &ScalingPolicy{DelaySeconds: 3600}}, false},
		{"delay over max", ContainerScalingOptions{ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 3601}}, true},
		{"cpu threshold below 5", ContainerScalingOptions{ScalingTriggers: &ScalingTriggers{CPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 4}}}, true},
		{"queue load threshold over 1000", ContainerScalingOptions{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{Threshold: 1000.5}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateDeploymentRequest_Validate_Limits(t *testing.T) {
	validRequest := func() CreateDeploymentRequest {
		return CreateDeploymentRequest{
			Name:       "test-deployment",
			Compute:    ContainerCompute{Name: "H100", Size: 1},
			Containers: []CreateDeploymentContainer{{Image: "nginx:1.25.3", ExposedPort: 80}},
		}
	}

	tests := []struct {
		name    string
		modify  func(r *CreateDeploymentRequest)
		wantErr bool
	}{
		{"valid", func(r *CreateDeploymentRequest) {}, false},
		{"compute size at max", func(r *CreateDeploymentRequest) { r.Compute.Size = 128 }, false},
		{"compute size over max", func(r *CreateDeploymentRequest) { r.Compute.Size = 129 }, true},
		{"exposed port at max", func(r *CreateDeploymentRequest) { r.Containers[0].ExposedPort = 65535 }, false},
		{"exposed port over max", func(r *CreateDeploymentRequest) { r.Containers[0].ExposedPort = 65536 }, true},
		{"healthcheck port at max", func(r *CreateDeploymentRequest) {
			r.Containers[0].Healthcheck = &ContainerHealthcheck{Enabled: true, Port: 65535, Path: "/health"}
		}, false},
		{"healthcheck port over max", func(r *CreateDeploymentRequest) {
			r.Containers[0].Healthcheck = &ContainerHealthcheck{Enabled: true, Port: 65536, Path: "/health"}
		}, true},
		{"disabled healthcheck port not checked", func(r *CreateDeploymentRequest) {
			r.Containers[0].Healthcheck = &ContainerHealthcheck{Enabled: false, Port: 65536}
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validRequest()
			tt.modify(&req)
			err := req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- Deployment names in request paths ---

type deploymentAction struct {
	name   string
	method string
	suffix string
	call   func(ctx context.Context, c *Client, deploymentName string) error
}

// deploymentActions lists every method that puts a deployment name in the request path.
func deploymentActions() []deploymentAction {
	envReq := &ContainerEnvVarsRequest{ContainerName: "app", Env: []ContainerEnvVar{{Type: "plain", Name: "A", ValueOrReferenceToSecret: "1"}}}
	return []deploymentAction{
		{"GetDeploymentByName", http.MethodGet, "", func(ctx context.Context, c *Client, n string) error {
			_, err := c.ContainerDeployments.GetDeploymentByName(ctx, n)
			return err
		}},
		{"UpdateDeployment", http.MethodPatch, "", func(ctx context.Context, c *Client, n string) error {
			_, err := c.ContainerDeployments.UpdateDeployment(ctx, n, &UpdateDeploymentRequest{})
			return err
		}},
		{"DeleteDeployment", http.MethodDelete, "", func(ctx context.Context, c *Client, n string) error {
			return c.ContainerDeployments.DeleteDeployment(ctx, n, -1)
		}},
		{"GetDeploymentStatus", http.MethodGet, "/status", func(ctx context.Context, c *Client, n string) error {
			_, err := c.ContainerDeployments.GetDeploymentStatus(ctx, n)
			return err
		}},
		{"RestartDeployment", http.MethodPost, "/restart", func(ctx context.Context, c *Client, n string) error {
			return c.ContainerDeployments.RestartDeployment(ctx, n)
		}},
		{"PauseDeployment", http.MethodPost, "/pause", func(ctx context.Context, c *Client, n string) error {
			return c.ContainerDeployments.PauseDeployment(ctx, n)
		}},
		{"ResumeDeployment", http.MethodPost, "/resume", func(ctx context.Context, c *Client, n string) error {
			return c.ContainerDeployments.ResumeDeployment(ctx, n)
		}},
		{"PurgeDeploymentQueue", http.MethodPost, "/purge-queue", func(ctx context.Context, c *Client, n string) error {
			return c.ContainerDeployments.PurgeDeploymentQueue(ctx, n)
		}},
		{"GetDeploymentScaling", http.MethodGet, "/scaling", func(ctx context.Context, c *Client, n string) error {
			_, err := c.ContainerDeployments.GetDeploymentScaling(ctx, n)
			return err
		}},
		{"UpdateDeploymentScaling", http.MethodPatch, "/scaling", func(ctx context.Context, c *Client, n string) error {
			_, err := c.ContainerDeployments.UpdateDeploymentScaling(ctx, n, &UpdateScalingOptionsRequest{MaxReplicaCount: new(1)})
			return err
		}},
		{"GetDeploymentReplicas", http.MethodGet, "/replicas", func(ctx context.Context, c *Client, n string) error {
			_, err := c.ContainerDeployments.GetDeploymentReplicas(ctx, n)
			return err
		}},
		{"GetEnvironmentVariables", http.MethodGet, "/environment-variables", func(ctx context.Context, c *Client, n string) error {
			_, err := c.ContainerDeployments.GetEnvironmentVariables(ctx, n)
			return err
		}},
		{"AddEnvironmentVariables", http.MethodPost, "/environment-variables", func(ctx context.Context, c *Client, n string) error {
			return c.ContainerDeployments.AddEnvironmentVariables(ctx, n, envReq)
		}},
		{"UpdateEnvironmentVariables", http.MethodPatch, "/environment-variables", func(ctx context.Context, c *Client, n string) error {
			return c.ContainerDeployments.UpdateEnvironmentVariables(ctx, n, envReq)
		}},
		{"DeleteEnvironmentVariables", http.MethodDelete, "/environment-variables", func(ctx context.Context, c *Client, n string) error {
			return c.ContainerDeployments.DeleteEnvironmentVariables(ctx, n, &DeleteContainerEnvVarsRequest{ContainerName: "app", Env: []string{"A"}})
		}},
	}
}

// Each method must build its path with buildDeploymentPath: the name is escaped on the wire,
// and an invalid name is rejected before any request is sent. TestBuildDeploymentPath covers
// every escaping and validation case; this test only proves that each method uses it.
func TestContainerDeploymentsService_DeploymentNameInPath(t *testing.T) {
	// Unescaped, this name would turn any action on "other-app" into a restart.
	const hostileName = "other-app/restart?x=1#frag"
	const wantEscaped = "/container-deployments/other-app%2Frestart%3Fx=1%23frag"

	for _, action := range deploymentActions() {
		t.Run(action.name, func(t *testing.T) {
			mockServer := testutil.NewMockServer()
			defer mockServer.Close()

			called := false
			// The mock server routes on the decoded path, so the handler uses the unescaped name.
			// The EscapedPath check is what proves the escaping on the wire.
			mockServer.SetHandler(action.method, "/container-deployments/"+hostileName+action.suffix, func(w http.ResponseWriter, r *http.Request) {
				called = true
				if got, want := r.URL.EscapedPath(), wantEscaped+action.suffix; got != want {
					t.Errorf("path = %s, want %s", got, want)
				}
				if r.URL.RawQuery != "" {
					t.Errorf("query = %q, want none", r.URL.RawQuery)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			client := NewTestClient(t, mockServer)
			ctx := context.Background()

			if err := action.call(ctx, client, hostileName); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !called {
				t.Error("request did not reach the escaped deployment path")
			}

			// ".." survives escaping, so the method must reject it before it sends a request.
			if err := action.call(ctx, client, ".."); err == nil || !strings.Contains(err.Error(), "deploymentName") {
				t.Errorf(`"..": error = %v, want a deploymentName validation error`, err)
			}
		})
	}
}

func TestBuildDeploymentPath(t *testing.T) {
	tests := []struct {
		name           string
		deploymentName string
		suffix         string
		want           string
		wantErr        string // substring of the expected error; empty means no error
	}{
		// Plain names are used as they are.
		{name: "name only", deploymentName: "my-app", want: "/container-deployments/my-app"},
		{name: "name with suffix", deploymentName: "my-app", suffix: "/scaling", want: "/container-deployments/my-app/scaling"},
		{name: "63 characters", deploymentName: strings.Repeat("a", 63), want: "/container-deployments/" + strings.Repeat("a", 63)},

		// Characters that would change the URL are escaped, so the name stays one path segment.
		{name: "slash cannot select another endpoint", deploymentName: "other-app/restart", suffix: "/purge-queue", want: "/container-deployments/other-app%2Frestart/purge-queue"},
		{name: "question mark cannot start a query", deploymentName: "app?force=true", suffix: "/pause", want: "/container-deployments/app%3Fforce=true/pause"},
		{name: "hash cannot start a fragment", deploymentName: "app#x", suffix: "/status", want: "/container-deployments/app%23x/status"},
		{name: "dot segments inside a name are escaped", deploymentName: "app/../other-app", want: "/container-deployments/app%2F..%2Fother-app"},
		{name: "length counts characters, not bytes", deploymentName: strings.Repeat("ä", 63), want: "/container-deployments/" + strings.Repeat("%C3%A4", 63)},

		// Names the API cannot accept are rejected before a path is built.
		{name: "empty name", deploymentName: "", suffix: "/scaling", wantErr: "deploymentName is required"},
		{name: "64 characters", deploymentName: strings.Repeat("a", 64), wantErr: "at most 63 characters"},
		{name: "64 non-ASCII characters", deploymentName: strings.Repeat("ä", 64), wantErr: "at most 63 characters"},
		{name: "single dot", deploymentName: ".", wantErr: "is not a valid name"},
		{name: "double dot", deploymentName: "..", suffix: "/restart", wantErr: "is not a valid name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildDeploymentPath(tt.deploymentName, tt.suffix)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildDeploymentPath(%q, %q) error = %v, want error containing %q", tt.deploymentName, tt.suffix, err, tt.wantErr)
				}
				if got != "" {
					t.Errorf("buildDeploymentPath(%q, %q) = %q, want empty path on error", tt.deploymentName, tt.suffix, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildDeploymentPath(%q, %q) unexpected error: %v", tt.deploymentName, tt.suffix, err)
			}
			if got != tt.want {
				t.Errorf("buildDeploymentPath(%q, %q) = %q, want %q", tt.deploymentName, tt.suffix, got, tt.want)
			}
		})
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
