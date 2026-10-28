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
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// maxDeploymentNameLength is the longest deployment name the API accepts.
const maxDeploymentNameLength = 63

// Limits the API enforces on deployment and job requests.
const (
	maxContainerPort                    = 65535
	maxComputeSize                      = 128
	maxReplicaCount                     = 1000
	maxConcurrentRequestsPerReplica     = 5000
	maxScalingDelaySeconds              = 3600  // 1 hour
	maxDeploymentQueueMessageTTLSeconds = 86400 // 1 day
)

// ContainerDeployment represents a serverless container deployment
type ContainerDeployment struct {
	Name                      string                     `json:"name"`
	Containers                []DeploymentContainer      `json:"containers"`
	EndpointBaseURL           string                     `json:"endpoint_base_url"`
	CreatedAt                 time.Time                  `json:"created_at"`
	Compute                   *ContainerCompute          `json:"compute,omitempty"`
	ContainerRegistrySettings *ContainerRegistrySettings `json:"container_registry_settings,omitempty"`
	IsSpot                    bool                       `json:"is_spot"`
}

// TargetNode represents the compute node/GPU configuration
type TargetNode struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}

// ContainerRegistrySettings represents registry authentication settings
type ContainerRegistrySettings struct {
	IsPrivate   bool                    `json:"is_private"`
	Credentials *RegistryCredentialsRef `json:"credentials"`
}

// RegistryCredentialsRef references registry credentials by name
type RegistryCredentialsRef struct {
	Name string `json:"name"`
}

// DeploymentContainer represents a container configuration in a deployment response
type DeploymentContainer struct {
	Image               ContainerImage                `json:"image"`
	Name                string                        `json:"name"`
	ExposedPort         int                           `json:"exposed_port"`
	Healthcheck         *ContainerHealthcheck         `json:"healthcheck,omitempty"`
	EntrypointOverrides *ContainerEntrypointOverrides `json:"entrypoint_overrides,omitempty"`
	Env                 []ContainerEnvVar             `json:"env"`
	VolumeMounts        []ContainerVolumeMount        `json:"volume_mounts"`
	AutoUpdate          *ContainerAutoUpdate          `json:"autoupdate,omitempty"`
}

// ContainerImage represents a container image reference
type ContainerImage struct {
	Image         string    `json:"image"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

// ContainerEnvVar represents an environment variable with type
type ContainerEnvVar struct {
	Type                     string `json:"type"`
	Name                     string `json:"name"`
	ValueOrReferenceToSecret string `json:"value_or_reference_to_secret"`
}

// DeploymentScalingOptions represents scaling configuration for container deployment
type DeploymentScalingOptions struct {
	DeadlineSeconds        int `json:"deadline_seconds,omitempty"`
	MaxReplicaCount        int `json:"max_replica_count"`
	QueueMessageTTLSeconds int `json:"queue_message_ttl_seconds,omitempty"`
}

// ContainerCompute represents compute resources for deployments
type ContainerCompute struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}

// CreateDeploymentRequest represents a request to create a new deployment
type CreateDeploymentRequest struct {
	Name                      string                      `json:"name"`
	IsSpot                    bool                        `json:"is_spot"`
	IsInferno                 bool                        `json:"is_inferno"`
	Compute                   ContainerCompute            `json:"compute"`
	ContainerRegistrySettings ContainerRegistrySettings   `json:"container_registry_settings"`
	Scaling                   ContainerScalingOptions     `json:"scaling"`
	Containers                []CreateDeploymentContainer `json:"containers"`
}

// CreateDeploymentContainer represents a container configuration for create/update requests
type CreateDeploymentContainer struct {
	Name                string                        `json:"name,omitempty"`
	Image               string                        `json:"image"`
	ExposedPort         int                           `json:"exposed_port,omitempty"`
	Healthcheck         *ContainerHealthcheck         `json:"healthcheck,omitempty"`
	EntrypointOverrides *ContainerEntrypointOverrides `json:"entrypoint_overrides,omitempty"`
	Env                 []ContainerEnvVar             `json:"env,omitempty"`
	VolumeMounts        []ContainerVolumeMount        `json:"volume_mounts,omitempty"`
	AutoUpdate          *ContainerAutoUpdate          `json:"autoupdate,omitempty"`
}

// UpdateDeploymentRequest represents a request to update a deployment
type UpdateDeploymentRequest struct {
	IsSpot                    *bool                       `json:"is_spot,omitempty"`
	IsInferno                 *bool                       `json:"is_inferno,omitempty"`
	Compute                   *ContainerCompute           `json:"compute,omitempty"`
	ContainerRegistrySettings *ContainerRegistrySettings  `json:"container_registry_settings,omitempty"`
	Scaling                   *ContainerScalingOptions    `json:"scaling,omitempty"`
	Containers                []CreateDeploymentContainer `json:"containers,omitempty"`
}

// ContainerDeploymentStatus represents the status of a container deployment
type ContainerDeploymentStatus struct {
	Status string `json:"status"`
}

// Deployment statuses returned by GetDeploymentStatus.
const (
	ContainerDeploymentStatusInitializing = "initializing"
	ContainerDeploymentStatusHealthy      = "healthy"
	ContainerDeploymentStatusDegraded     = "degraded"
	ContainerDeploymentStatusUnhealthy    = "unhealthy"
	ContainerDeploymentStatusPaused       = "paused"
	ContainerDeploymentStatusQuotaReached = "quota_reached"
	ContainerDeploymentStatusImagePulling = "image_pulling"
	ContainerDeploymentStatusUpdating     = "updating"
	ContainerDeploymentStatusTerminating  = "terminating"
)

// ContainerScalingOptions represents scaling configuration
type ContainerScalingOptions struct {
	MinReplicaCount              int              `json:"min_replica_count"`
	MaxReplicaCount              int              `json:"max_replica_count"`
	ScaleDownPolicy              *ScalingPolicy   `json:"scale_down_policy"`
	ScaleUpPolicy                *ScalingPolicy   `json:"scale_up_policy"`
	QueueMessageTTLSeconds       int              `json:"queue_message_ttl_seconds"`
	ConcurrentRequestsPerReplica int              `json:"concurrent_requests_per_replica"`
	ScalingTriggers              *ScalingTriggers `json:"scaling_triggers"`
}

// ScalingPolicy represents scale up/down policy configuration
type ScalingPolicy struct {
	DelaySeconds int `json:"delay_seconds"`
}

// ScalingTriggers represents the various scaling triggers.
// In UpdateDeploymentScaling, a nil trigger is left out of the request and keeps its current settings.
type ScalingTriggers struct {
	QueueLoad      *QueueLoadTrigger   `json:"queue_load,omitempty"`
	CPUUtilization *UtilizationTrigger `json:"cpu_utilization,omitempty"`
	GPUUtilization *UtilizationTrigger `json:"gpu_utilization,omitempty"`
}

// QueueLoadTrigger represents queue load based scaling trigger.
// Threshold is the number of queued requests per replica that triggers a scale-up,
// from 1 to 1000. Fractional values such as 1.5 are allowed.
type QueueLoadTrigger struct {
	Threshold float64 `json:"threshold"`
}

// UtilizationTrigger represents CPU/GPU utilization based scaling trigger
type UtilizationTrigger struct {
	Enabled   bool `json:"enabled"`
	Threshold int  `json:"threshold"`
}

// UpdateScalingOptionsRequest represents a PATCH request to update scaling options
type UpdateScalingOptionsRequest struct {
	MinReplicaCount              *int             `json:"min_replica_count,omitempty"`
	MaxReplicaCount              *int             `json:"max_replica_count,omitempty"`
	ScaleDownPolicy              *ScalingPolicy   `json:"scale_down_policy,omitempty"`
	ScaleUpPolicy                *ScalingPolicy   `json:"scale_up_policy,omitempty"`
	QueueMessageTTLSeconds       *int             `json:"queue_message_ttl_seconds,omitempty"`
	ConcurrentRequestsPerReplica *int             `json:"concurrent_requests_per_replica,omitempty"`
	ScalingTriggers              *ScalingTriggers `json:"scaling_triggers,omitempty"`
}

// DeploymentReplicas represents replica information for a deployment
type DeploymentReplicas struct {
	List []ReplicaInfo `json:"list"`
}

// ReplicaInfo represents information about a single replica
type ReplicaInfo struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	// Image is the full image reference the replica runs, e.g. "docker.io/library/nginx:1.21".
	Image     string `json:"image,omitempty"`
	ImageName string `json:"image_name,omitempty"`
	ImageTag  string `json:"image_tag,omitempty"`
}

// Replica statuses reported in ReplicaInfo.Status.
const (
	ReplicaStatusUnavailable  = "unavailable"
	ReplicaStatusInitializing = "initializing"
	ReplicaStatusRunning      = "running"
	ReplicaStatusTerminating  = "terminating"
	ReplicaStatusError        = "error"
	ReplicaStatusImagePulling = "imagepulling"
)

// ContainerEnvVarsRequest represents a request to add/update environment variables
type ContainerEnvVarsRequest struct {
	ContainerName string            `json:"container_name"`
	Env           []ContainerEnvVar `json:"env"`
}

// ContainerHealthcheck represents container healthcheck
type ContainerHealthcheck struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port,omitempty"`
	Path    string `json:"path,omitempty"`
}

// ContainerEntrypointOverrides includes functionality for overriding container entrypoint
type ContainerEntrypointOverrides struct {
	Enabled    bool     `json:"enabled"`
	Entrypoint []string `json:"entrypoint,omitempty"`
	Cmd        []string `json:"cmd,omitempty"`
}

// ContainerVolumeMount represents the container volume mount
type ContainerVolumeMount struct {
	Type       string `json:"type"`
	MountPath  string `json:"mount_path"`
	SecretName string `json:"secret_name,omitempty"`
	SizeInMB   int    `json:"size_in_mb,omitempty"`
	VolumeID   string `json:"volume_id,omitempty"`
}

// ContainerAutoUpdate has automatic update instructions
type ContainerAutoUpdate struct {
	Enabled   bool   `json:"enabled"`
	Mode      string `json:"mode"`
	TagFilter string `json:"tag_filter,omitempty"`
}

// DeleteContainerEnvVarsRequest represents a request to delete environment variables
type DeleteContainerEnvVarsRequest struct {
	ContainerName string   `json:"container_name"`
	Env           []string `json:"env"`
}

// ComputeResource represents available compute resources
type ComputeResource struct {
	Name        string `json:"name"`
	Size        int    `json:"size"`
	IsAvailable bool   `json:"is_available"`
}

// Secret represents a secret used in deployments
type Secret struct {
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	SecretType string    `json:"secret_type"`
}

// CreateSecretRequest represents a request to create a new secret
type CreateSecretRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// FileSecret represents a fileset secret
type FileSecret struct {
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	SecretType string    `json:"secret_type"`
	FileNames  []string  `json:"file_names"`
}

// CreateFileSecretRequest represents a request to create a fileset secret
type CreateFileSecretRequest struct {
	Name  string           `json:"name"`
	Files []FileSecretFile `json:"files"`
}

// FileSecretFile represents a file in a fileset secret
type FileSecretFile struct {
	Name          string `json:"file_name"`
	Base64Content string `json:"base64_content"`
}

// RegistryCredentials represents container registry credentials
type RegistryCredentials struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateRegistryCredentialsRequest represents a request to create registry credentials
type CreateRegistryCredentialsRequest struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	Username          string `json:"username,omitempty"`
	AccessToken       string `json:"access_token,omitempty"`
	ServiceAccountKey string `json:"service_account_key,omitempty"`
	DockerConfigJson  string `json:"docker_config_json,omitempty"`
	AccessKeyID       string `json:"access_key_id,omitempty"`
	SecretAccessKey   string `json:"secret_access_key,omitempty"`
	Region            string `json:"region,omitempty"`
	EcrRepo           string `json:"ecr_repo,omitempty"`
	ScalewayDomain    string `json:"scaleway_domain,omitempty"`
	ScalewayUUID      string `json:"scaleway_uuid,omitempty"`
}

// Validate validates the ContainerCompute fields
func (r ContainerCompute) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Size, validation.Required, validation.Min(1), validation.Max(maxComputeSize)),
	)
}

// Validate validates the CreateDeploymentContainer fields
func (r CreateDeploymentContainer) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Image, validation.Required),
		validation.Field(&r.ExposedPort, validation.Max(maxContainerPort)),
		validation.Field(&r.Healthcheck),
	)
}

// Validate validates the ContainerHealthcheck fields. Port is checked only when the healthcheck is enabled.
func (h ContainerHealthcheck) Validate() error {
	return validation.ValidateStruct(&h,
		validation.Field(&h.Port, validation.When(h.Enabled, validation.Max(maxContainerPort))),
	)
}

// Validate validates the CreateDeploymentRequest fields
func (r CreateDeploymentRequest) Validate() error {
	if err := validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Compute, validation.Required),
		validation.Field(&r.Containers, validation.Required, validation.Length(1, 0)),
	); err != nil {
		return err
	}
	for i, c := range r.Containers {
		if c.ExposedPort < 1 {
			return fmt.Errorf("containers[%d].exposed_port is required and must be >= 1", i)
		}
	}
	return nil
}

// validateDeploymentName checks a deployment name before it is used in a request path.
func validateDeploymentName(name string) error {
	if name == "" {
		return fmt.Errorf("deploymentName is required")
	}
	if utf8.RuneCountInString(name) > maxDeploymentNameLength {
		return fmt.Errorf("deploymentName must be at most %d characters", maxDeploymentNameLength)
	}
	// "." and ".." survive path escaping and would change which endpoint is called.
	if name == "." || name == ".." {
		return fmt.Errorf("deploymentName %q is not a valid name", name)
	}
	return nil
}

// Validate validates the UpdateScalingOptionsRequest fields.
// At least one option must be set, and min_replica_count must not exceed max_replica_count when both are set.
func (r UpdateScalingOptionsRequest) Validate() error {
	if r == (UpdateScalingOptionsRequest{}) {
		return fmt.Errorf("at least one scaling option must be set")
	}
	if err := r.validateLimits(); err != nil {
		return err
	}
	if r.MinReplicaCount != nil && r.MaxReplicaCount != nil && *r.MinReplicaCount > *r.MaxReplicaCount {
		return fmt.Errorf("min_replica_count (%d) must not be greater than max_replica_count (%d)", *r.MinReplicaCount, *r.MaxReplicaCount)
	}
	return nil
}

// validateLimits checks the scaling options that are set against the API limits.
func (r UpdateScalingOptionsRequest) validateLimits() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.MinReplicaCount, validation.Min(0), validation.Max(maxReplicaCount)),
		validation.Field(&r.MaxReplicaCount, validation.Min(0), validation.Max(maxReplicaCount)),
		validation.Field(&r.ScaleDownPolicy),
		validation.Field(&r.ScaleUpPolicy),
		validation.Field(&r.QueueMessageTTLSeconds, validation.Min(0), validation.Max(maxDeploymentQueueMessageTTLSeconds)),
		validation.Field(&r.ConcurrentRequestsPerReplica, validation.Min(0), validation.Max(maxConcurrentRequestsPerReplica)),
		validation.Field(&r.ScalingTriggers),
	)
}

// Validate validates the ContainerScalingOptions fields against the API limits.
func (o ContainerScalingOptions) Validate() error {
	return UpdateScalingOptionsRequest{
		MinReplicaCount:              &o.MinReplicaCount,
		MaxReplicaCount:              &o.MaxReplicaCount,
		ScaleDownPolicy:              o.ScaleDownPolicy,
		ScaleUpPolicy:                o.ScaleUpPolicy,
		QueueMessageTTLSeconds:       &o.QueueMessageTTLSeconds,
		ConcurrentRequestsPerReplica: &o.ConcurrentRequestsPerReplica,
		ScalingTriggers:              o.ScalingTriggers,
	}.validateLimits()
}

// Validate validates the ScalingPolicy fields
func (p ScalingPolicy) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.DelaySeconds, validation.Min(0), validation.Max(maxScalingDelaySeconds)),
	)
}

// Validate validates the triggers that are set
func (t ScalingTriggers) Validate() error {
	return validation.ValidateStruct(&t,
		validation.Field(&t.QueueLoad),
		validation.Field(&t.CPUUtilization),
		validation.Field(&t.GPUUtilization),
	)
}

// Validate validates the QueueLoadTrigger fields. Threshold must be from 1 to 1000.
func (t QueueLoadTrigger) Validate() error {
	return validation.ValidateStruct(&t,
		validation.Field(&t.Threshold, validation.Required, validation.Min(1.0), validation.Max(1000.0)),
	)
}

// Validate validates the UtilizationTrigger fields. Threshold is a percentage from 5 to 100,
// checked only when the trigger is enabled and the threshold is set.
func (t UtilizationTrigger) Validate() error {
	return validation.ValidateStruct(&t,
		validation.Field(&t.Threshold, validation.When(t.Enabled && t.Threshold != 0, validation.Min(5), validation.Max(100))),
	)
}

// Validate validates the CreateSecretRequest fields
func (r CreateSecretRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Value, validation.Required),
	)
}

// Validate validates the CreateFileSecretRequest fields
func (r CreateFileSecretRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Files, validation.Required, validation.Length(1, 0)),
	)
}

// Validate validates the CreateRegistryCredentialsRequest fields
func (r CreateRegistryCredentialsRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Type, validation.Required,
			validation.In("verda", "gcr", "dockerhub", "ghcr", "aws-ecr", "scaleway", "custom")),
	)
}

// ValidateCreateDeploymentRequest performs extended validation beyond Validate(),
// including image tag checks, scaling policy requirements and scaling limits.
func ValidateCreateDeploymentRequest(req *CreateDeploymentRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	for _, c := range req.Containers {
		if IsLatestTag(c.Image) {
			return fmt.Errorf("container image %q must use a specific tag, not 'latest'", c.Image)
		}
	}
	if req.Scaling.ScaleDownPolicy == nil {
		return fmt.Errorf("scaling.scale_down_policy is required")
	}
	if req.Scaling.ScaleUpPolicy == nil {
		return fmt.Errorf("scaling.scale_up_policy is required")
	}
	if err := req.Scaling.Validate(); err != nil {
		return fmt.Errorf("scaling: %w", err)
	}
	return nil
}
