package verda

import (
	"testing"
)

func TestIsLatestTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected bool
	}{
		// Should be detected as latest
		{"explicit latest tag", "nginx:latest", true},
		{"no tag defaults to latest", "nginx", true},
		{"registry with latest tag", "registry-1.docker.io/library/nginx:latest", true},
		{"no tag with registry", "registry-1.docker.io/library/nginx", true},

		// Should NOT be detected as latest
		{"specific version tag", "nginx:1.25.3", false},
		{"sha digest", "nginx@sha256:abc123", false},
		{"registry with version", "registry-1.docker.io/library/nginx:1.25.3", false},
		{"alpine with version", "alpine:3.19", false},
		{"python with version", "python:3.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLatestTag(tt.image)
			if result != tt.expected {
				t.Errorf("isLatestTag(%q) = %v, want %v", tt.image, result, tt.expected)
			}
		})
	}
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
		err := validateCreateDeploymentRequest(req)
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
		err := validateCreateDeploymentRequest(req)
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
		err := validateCreateDeploymentRequest(req)
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
				// ScaleDownPolicy missing
				ScaleUpPolicy:   &ScalingPolicy{DelaySeconds: 60},
				ScalingTriggers: &ScalingTriggers{},
			},
		}
		err := validateCreateDeploymentRequest(req)
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
					QueueLoad: &QueueLoadTrigger{Threshold: 0.5}, // Must be >= 1
				},
			},
		}
		err := validateCreateDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for queue_load threshold < 1, got nil")
		}
	})
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
				{
					Image: "alpine:3.19",
				},
			},
			Scaling: &JobScalingOptions{
				MaxReplicaCount:        1,
				QueueMessageTTLSeconds: 300,
				DeadlineSeconds:        3600,
			},
		}
		err := validateCreateJobDeploymentRequest(req)
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
				{
					Image: "alpine:3.19",
				},
			},
			Scaling: &JobScalingOptions{
				MaxReplicaCount:        1,
				QueueMessageTTLSeconds: 300,
				// DeadlineSeconds missing
			},
		}
		err := validateCreateJobDeploymentRequest(req)
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
				{
					Image: "alpine:latest",
				},
			},
			Scaling: &JobScalingOptions{
				MaxReplicaCount:        1,
				QueueMessageTTLSeconds: 300,
				DeadlineSeconds:        3600,
			},
		}
		err := validateCreateJobDeploymentRequest(req)
		if err == nil {
			t.Error("expected error for latest tag, got nil")
		}
	})
}
