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
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// JobDeploymentShortInfo represents summary information about a job deployment
type JobDeploymentShortInfo struct {
	Name      string            `json:"name"`
	CreatedAt time.Time         `json:"created_at"`
	Compute   *ContainerCompute `json:"compute"`
}

// JobDeployment represents a complete serverless job deployment
type JobDeployment struct {
	Name                      string                     `json:"name"`
	Containers                []DeploymentContainer      `json:"containers"`
	EndpointBaseURL           string                     `json:"endpoint_base_url"`
	CreatedAt                 time.Time                  `json:"created_at"`
	Compute                   *ContainerCompute          `json:"compute"`
	ContainerRegistrySettings *ContainerRegistrySettings `json:"container_registry_settings"`
	Scaling                   *JobScalingOptions         `json:"scaling,omitempty"`
}

// CreateJobDeploymentRequest represents a request to create a new job deployment
type CreateJobDeploymentRequest struct {
	Name                      string                      `json:"name"`
	ContainerRegistrySettings *ContainerRegistrySettings  `json:"container_registry_settings,omitempty"`
	Containers                []CreateDeploymentContainer `json:"containers"`
	Compute                   *ContainerCompute           `json:"compute,omitempty"`
	Scaling                   *JobScalingOptions          `json:"scaling,omitempty"`
}

// UpdateJobDeploymentRequest represents a request to update a job deployment
type UpdateJobDeploymentRequest struct {
	ContainerRegistrySettings *ContainerRegistrySettings  `json:"container_registry_settings,omitempty"`
	Containers                []CreateDeploymentContainer `json:"containers,omitempty"`
	Compute                   *ContainerCompute           `json:"compute,omitempty"`
	Scaling                   *JobScalingOptions          `json:"scaling,omitempty"`
}

// JobScalingOptions represents scaling configuration for a job deployment
type JobScalingOptions struct {
	MaxReplicaCount        int `json:"max_replica_count"`
	QueueMessageTTLSeconds int `json:"queue_message_ttl_seconds,omitempty"`
	DeadlineSeconds        int `json:"deadline_seconds,omitempty"`
}

// JobDeploymentStatus represents the status of a job deployment
type JobDeploymentStatus struct {
	Status string `json:"status"`
}

// Validate validates the CreateJobDeploymentRequest fields
func (r CreateJobDeploymentRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Containers, validation.Required, validation.Length(1, 0)),
		validation.Field(&r.Compute, validation.Required),
		validation.Field(&r.Scaling, validation.Required),
	)
}

// ValidateCreateJobDeploymentRequest performs extended validation beyond Validate(),
// including image tag checks and scaling deadline requirements.
func ValidateCreateJobDeploymentRequest(req *CreateJobDeploymentRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	for _, c := range req.Containers {
		if IsLatestTag(c.Image) {
			return fmt.Errorf("container image %q must use a specific tag, not 'latest'", c.Image)
		}
	}
	if req.Scaling != nil && req.Scaling.DeadlineSeconds <= 0 {
		return fmt.Errorf("scaling.deadline_seconds is required and must be > 0")
	}
	return nil
}
