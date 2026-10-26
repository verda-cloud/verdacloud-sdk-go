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
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestServerlessJobsService_GetJobDeployments(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	t.Run("get all job deployments", func(t *testing.T) {
		ctx := context.Background()
		jobs, err := client.ServerlessJobs.GetJobDeployments(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(jobs) == 0 {
			t.Fatal("expected at least one job deployment")
		}

		job := jobs[0]
		if job.Name == "" {
			t.Error("expected job to have a Name")
		}
		if job.CreatedAt.IsZero() {
			t.Error("expected job to have a CreatedAt")
		}
		if job.Compute == nil {
			t.Error("expected job to have Compute")
		}
	})
}

func TestServerlessJobsService_CreateJobDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	t.Run("create job deployment", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateJobDeploymentRequest{
			Name: "flux-training",
			ContainerRegistrySettings: &ContainerRegistrySettings{
				Credentials: &RegistryCredentialsRef{
					Name: "dockerhub-credentials",
				},
			},
			Containers: []CreateDeploymentContainer{
				{
					Image:       "registry-1.docker.io/chentex/random-logger:v1.0.1",
					ExposedPort: 8080,
				},
			},
			Compute: &ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Scaling: &JobScalingOptions{
				MaxReplicaCount:        1,
				QueueMessageTTLSeconds: 300,
				DeadlineSeconds:        3600,
			},
		}

		job, err := client.ServerlessJobs.CreateJobDeployment(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job == nil {
			t.Fatal("expected job, got nil")
		}
		if job.Name == "" {
			t.Error("expected job to have a Name")
		}
		if job.CreatedAt.IsZero() {
			t.Error("expected job to have a CreatedAt")
		}
	})

	t.Run("validation - nil request", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.ServerlessJobs.CreateJobDeployment(ctx, nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("validation - empty name", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateJobDeploymentRequest{
			Name: "",
			Containers: []CreateDeploymentContainer{
				{Image: "nginx:1.25"},
			},
			Compute: &ContainerCompute{Name: "H100", Size: 1},
			Scaling: &JobScalingOptions{MaxReplicaCount: 1, DeadlineSeconds: 3600},
		}
		_, err := client.ServerlessJobs.CreateJobDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("validation - no containers", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateJobDeploymentRequest{
			Name:       "test-job",
			Containers: []CreateDeploymentContainer{},
			Compute:    &ContainerCompute{Name: "H100", Size: 1},
			Scaling:    &JobScalingOptions{MaxReplicaCount: 1, DeadlineSeconds: 3600},
		}
		_, err := client.ServerlessJobs.CreateJobDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for empty containers")
		}
	})

	t.Run("validation - missing compute", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateJobDeploymentRequest{
			Name: "test-job",
			Containers: []CreateDeploymentContainer{
				{Image: "nginx:1.25"},
			},
			Scaling: &JobScalingOptions{MaxReplicaCount: 1, DeadlineSeconds: 3600},
		}
		_, err := client.ServerlessJobs.CreateJobDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for missing compute")
		}
	})

	t.Run("validation - missing scaling", func(t *testing.T) {
		ctx := context.Background()
		req := &CreateJobDeploymentRequest{
			Name: "test-job",
			Containers: []CreateDeploymentContainer{
				{Image: "nginx:1.25"},
			},
			Compute: &ContainerCompute{Name: "H100", Size: 1},
		}
		_, err := client.ServerlessJobs.CreateJobDeployment(ctx, req)
		if err == nil {
			t.Error("expected error for missing scaling")
		}
	})
}

func TestServerlessJobsService_GetJobDeploymentByName(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	job, err := client.ServerlessJobs.GetJobDeploymentByName(ctx, "flux-training")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("expected job, got nil")
	}
	if job.Name != "flux-training" {
		t.Errorf("expected name 'flux-training', got '%s'", job.Name)
	}
	if len(job.Containers) == 0 {
		t.Error("expected job to have containers")
	}
}

func TestServerlessJobsService_UpdateJobDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	t.Run("validation - nil request", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.ServerlessJobs.UpdateJobDeployment(ctx, "test-job", nil)
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("validation - empty job name", func(t *testing.T) {
		ctx := context.Background()
		req := &UpdateJobDeploymentRequest{
			Scaling: &JobScalingOptions{MaxReplicaCount: 2, DeadlineSeconds: 3600},
		}
		_, err := client.ServerlessJobs.UpdateJobDeployment(ctx, "", req)
		if err == nil {
			t.Error("expected error for empty job name")
		}
	})

	t.Run("partial update - scaling only", func(t *testing.T) {
		ctx := context.Background()
		req := &UpdateJobDeploymentRequest{
			Scaling: &JobScalingOptions{MaxReplicaCount: 2, QueueMessageTTLSeconds: 300, DeadlineSeconds: 3600},
		}
		job, err := client.ServerlessJobs.UpdateJobDeployment(ctx, "test-job", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job == nil {
			t.Fatal("expected job, got nil")
		}
	})

	t.Run("container update - with name", func(t *testing.T) {
		ctx := context.Background()
		req := &UpdateJobDeploymentRequest{
			Containers: []CreateDeploymentContainer{
				{
					Name:        "random-logger-0",
					Image:       "registry-1.docker.io/chentex/random-logger:v1.0.1",
					ExposedPort: 8080,
				},
			},
		}
		job, err := client.ServerlessJobs.UpdateJobDeployment(ctx, "test-job", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job == nil {
			t.Fatal("expected job, got nil")
		}
	})

	t.Run("validation - updating container without name", func(t *testing.T) {
		ctx := context.Background()
		req := &UpdateJobDeploymentRequest{
			Containers: []CreateDeploymentContainer{
				{Image: "nginx:1.25", ExposedPort: 8080},
			},
		}
		_, err := client.ServerlessJobs.UpdateJobDeployment(ctx, "test-job", req)
		if err == nil {
			t.Error("expected error when container name is missing")
		}
	})
}

func TestServerlessJobsService_DeleteJobDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("timeout 0 - skip waiting", func(t *testing.T) {
		err := client.ServerlessJobs.DeleteJobDeployment(ctx, "test-job", 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("timeout negative - use API default", func(t *testing.T) {
		err := client.ServerlessJobs.DeleteJobDeployment(ctx, "test-job", -1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("timeout 5000ms", func(t *testing.T) {
		err := client.ServerlessJobs.DeleteJobDeployment(ctx, "test-job", 5000)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("timeout capped at 300000", func(t *testing.T) {
		err := client.ServerlessJobs.DeleteJobDeployment(ctx, "test-job", 400000)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestServerlessJobsService_GetJobDeploymentScaling(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		scaling, err := client.ServerlessJobs.GetJobDeploymentScaling(ctx, "test-job")
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
		_, err := client.ServerlessJobs.GetJobDeploymentScaling(ctx, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestServerlessJobsService_GetJobDeploymentStatus(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	ctx := context.Background()
	status, err := client.ServerlessJobs.GetJobDeploymentStatus(ctx, "test-job")
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

func TestServerlessJobsService_PurgeJobDeploymentQueue(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	err := client.ServerlessJobs.PurgeJobDeploymentQueue(context.Background(), "test-job")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestServerlessJobsService_PauseJobDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	err := client.ServerlessJobs.PauseJobDeployment(context.Background(), "test-job")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestServerlessJobsService_ResumeJobDeployment(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(t, mockServer)

	err := client.ServerlessJobs.ResumeJobDeployment(context.Background(), "test-job")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateCreateJobDeploymentRequest(t *testing.T) {
	t.Run("valid request passes", func(t *testing.T) {
		req := &CreateJobDeploymentRequest{
			Name: "test-job",
			Compute: &ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{Image: "alpine:3.19"},
			},
			Scaling: &JobScalingOptions{
				MaxReplicaCount:        1,
				QueueMessageTTLSeconds: 300,
				DeadlineSeconds:        3600,
			},
		}
		err := ValidateCreateJobDeploymentRequest(req)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("missing deadline_seconds rejected", func(t *testing.T) {
		req := &CreateJobDeploymentRequest{
			Name: "test-job",
			Compute: &ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{Image: "alpine:3.19"},
			},
			Scaling: &JobScalingOptions{
				MaxReplicaCount:        1,
				QueueMessageTTLSeconds: 300,
			},
		}
		err := ValidateCreateJobDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for missing deadline_seconds, got nil")
		}
	})

	t.Run("latest tag rejected", func(t *testing.T) {
		req := &CreateJobDeploymentRequest{
			Name: "test-job",
			Compute: &ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{Image: "alpine:latest"},
			},
			Scaling: &JobScalingOptions{
				MaxReplicaCount:        1,
				QueueMessageTTLSeconds: 300,
				DeadlineSeconds:        3600,
			},
		}
		err := ValidateCreateJobDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for latest tag, got nil")
		}
	})

	t.Run("no tag rejected (defaults to latest)", func(t *testing.T) {
		req := &CreateJobDeploymentRequest{
			Name: "test-job",
			Compute: &ContainerCompute{
				Name: "H100",
				Size: 1,
			},
			Containers: []CreateDeploymentContainer{
				{Image: "alpine"},
			},
			Scaling: &JobScalingOptions{
				MaxReplicaCount:        1,
				QueueMessageTTLSeconds: 300,
				DeadlineSeconds:        3600,
			},
		}
		err := ValidateCreateJobDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for image without tag, got nil")
		}
	})
}

func TestCreateJobDeploymentRequest_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		req := CreateJobDeploymentRequest{
			Name:       "test-job",
			Containers: []CreateDeploymentContainer{{Image: "nginx:1.25"}},
			Compute:    &ContainerCompute{Name: "H100", Size: 1},
			Scaling:    &JobScalingOptions{MaxReplicaCount: 1, DeadlineSeconds: 3600},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		req := CreateJobDeploymentRequest{
			Name:       "",
			Containers: []CreateDeploymentContainer{{Image: "nginx:1.25"}},
			Compute:    &ContainerCompute{Name: "H100", Size: 1},
			Scaling:    &JobScalingOptions{MaxReplicaCount: 1, DeadlineSeconds: 3600},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("no containers", func(t *testing.T) {
		req := CreateJobDeploymentRequest{
			Name:       "test-job",
			Containers: []CreateDeploymentContainer{},
			Compute:    &ContainerCompute{Name: "H100", Size: 1},
			Scaling:    &JobScalingOptions{MaxReplicaCount: 1, DeadlineSeconds: 3600},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty containers")
		}
	})

	t.Run("nil compute", func(t *testing.T) {
		req := CreateJobDeploymentRequest{
			Name:       "test-job",
			Containers: []CreateDeploymentContainer{{Image: "nginx:1.25"}},
			Scaling:    &JobScalingOptions{MaxReplicaCount: 1, DeadlineSeconds: 3600},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for nil compute")
		}
	})

	t.Run("nil scaling", func(t *testing.T) {
		req := CreateJobDeploymentRequest{
			Name:       "test-job",
			Containers: []CreateDeploymentContainer{{Image: "nginx:1.25"}},
			Compute:    &ContainerCompute{Name: "H100", Size: 1},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for nil scaling")
		}
	})
}
