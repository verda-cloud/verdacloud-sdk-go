package verda

import (
	"context"
	"fmt"
	"net/url"
)

type ServerlessJobsService struct {
	client *Client
}

func (s *ServerlessJobsService) GetJobDeployments(ctx context.Context) ([]JobDeploymentShortInfo, error) {
	jobs, _, err := getRequest[[]JobDeploymentShortInfo](ctx, s.client, "/job-deployments")
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *ServerlessJobsService) CreateJobDeployment(ctx context.Context, req *CreateJobDeploymentRequest) (*JobDeployment, error) {
	if err := validateCreateJobDeploymentRequest(req); err != nil {
		return nil, err
	}

	job, _, err := postRequest[JobDeployment](ctx, s.client, "/job-deployments", req)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// validateCreateJobDeploymentRequest validates all required fields for job deployment creation
func validateCreateJobDeploymentRequest(req *CreateJobDeploymentRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	// Basic required fields
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.Compute == nil {
		return fmt.Errorf("compute is required")
	}
	if req.Compute.Name == "" {
		return fmt.Errorf("compute.name is required")
	}

	// Container validation
	if len(req.Containers) == 0 {
		return fmt.Errorf("at least one container is required")
	}
	for i, c := range req.Containers {
		if c.Image == "" {
			return fmt.Errorf("containers[%d].image is required", i)
		}
		// Check for "latest" tag - API does not allow it
		if isLatestTag(c.Image) {
			return fmt.Errorf("containers[%d].image: 'latest' tag is not allowed, please specify a specific version tag (e.g., alpine:3.19)", i)
		}
	}

	// Scaling validation
	if req.Scaling == nil {
		return fmt.Errorf("scaling is required")
	}
	if req.Scaling.MaxReplicaCount == 0 {
		return fmt.Errorf("scaling.max_replica_count is required")
	}
	if req.Scaling.DeadlineSeconds == 0 {
		return fmt.Errorf("scaling.deadline_seconds is required (job timeout in seconds)")
	}
	if req.Scaling.QueueMessageTTLSeconds == 0 {
		return fmt.Errorf("scaling.queue_message_ttl_seconds is required")
	}

	return nil
}

func (s *ServerlessJobsService) GetJobDeploymentByName(ctx context.Context, jobName string) (*JobDeployment, error) {
	path := fmt.Sprintf("/job-deployments/%s", jobName)
	job, _, err := getRequest[JobDeployment](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *ServerlessJobsService) UpdateJobDeployment(ctx context.Context, jobName string, req *UpdateJobDeploymentRequest) (*JobDeployment, error) {
	path := fmt.Sprintf("/job-deployments/%s", jobName)
	job, _, err := patchRequest[JobDeployment](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// DeleteJobDeployment removes a job with optional timeout in milliseconds
func (s *ServerlessJobsService) DeleteJobDeployment(ctx context.Context, jobName string, timeoutMs int) error {
	path := fmt.Sprintf("/job-deployments/%s", jobName)

	if timeoutMs > 0 {
		params := url.Values{}
		params.Set("timeout", fmt.Sprintf("%d", timeoutMs))
		path += "?" + params.Encode()
	}

	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}

func (s *ServerlessJobsService) GetJobDeploymentScaling(ctx context.Context, jobName string) (*JobScalingOptions, error) {
	if jobName == "" {
		return nil, fmt.Errorf("jobName is required")
	}
	path := fmt.Sprintf("/job-deployments/%s/scaling", jobName)
	scaling, _, err := getRequest[JobScalingOptions](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &scaling, nil
}

func (s *ServerlessJobsService) UpdateJobDeploymentScaling(ctx context.Context, jobName string, req *UpdateScalingOptionsRequest) (*JobScalingOptions, error) {
	if jobName == "" {
		return nil, fmt.Errorf("jobName is required")
	}
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	path := fmt.Sprintf("/job-deployments/%s/scaling", jobName)
	scaling, _, err := patchRequest[JobScalingOptions](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &scaling, nil
}

func (s *ServerlessJobsService) PurgeJobDeploymentQueue(ctx context.Context, jobName string) error {
	path := fmt.Sprintf("/job-deployments/%s/purge-queue", jobName)
	_, _, err := postRequest[interface{}](ctx, s.client, path, nil)
	return err
}

func (s *ServerlessJobsService) PauseJobDeployment(ctx context.Context, jobName string) error {
	path := fmt.Sprintf("/job-deployments/%s/pause", jobName)
	_, _, err := postRequest[interface{}](ctx, s.client, path, nil)
	return err
}

func (s *ServerlessJobsService) ResumeJobDeployment(ctx context.Context, jobName string) error {
	path := fmt.Sprintf("/job-deployments/%s/resume", jobName)
	_, _, err := postRequest[interface{}](ctx, s.client, path, nil)
	return err
}

func (s *ServerlessJobsService) GetJobDeploymentStatus(ctx context.Context, jobName string) (*JobDeploymentStatus, error) {
	path := fmt.Sprintf("/job-deployments/%s/status", jobName)
	status, _, err := getRequest[JobDeploymentStatus](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *ServerlessJobsService) GetJobEnvironmentVariables(ctx context.Context, jobName string) ([]ContainerEnvVar, error) {
	if jobName == "" {
		return nil, fmt.Errorf("jobName is required")
	}
	path := fmt.Sprintf("/job-deployments/%s/environment-variables", jobName)
	envVars, _, err := getRequest[[]ContainerEnvVar](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return envVars, nil
}

func (s *ServerlessJobsService) AddJobEnvironmentVariables(ctx context.Context, jobName string, req *EnvironmentVariablesRequest) error {
	if jobName == "" {
		return fmt.Errorf("jobName is required")
	}
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if req.ContainerName == "" {
		return fmt.Errorf("container_name is required")
	}
	if len(req.Env) == 0 {
		return fmt.Errorf("env array cannot be empty")
	}
	path := fmt.Sprintf("/job-deployments/%s/environment-variables", jobName)
	_, _, err := postRequest[interface{}](ctx, s.client, path, req)
	return err
}

func (s *ServerlessJobsService) UpdateJobEnvironmentVariables(ctx context.Context, jobName string, req *EnvironmentVariablesRequest) error {
	if jobName == "" {
		return fmt.Errorf("jobName is required")
	}
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if req.ContainerName == "" {
		return fmt.Errorf("container_name is required")
	}
	if len(req.Env) == 0 {
		return fmt.Errorf("env array cannot be empty")
	}
	path := fmt.Sprintf("/job-deployments/%s/environment-variables", jobName)
	_, _, err := patchRequest[interface{}](ctx, s.client, path, req)
	return err
}

func (s *ServerlessJobsService) DeleteJobEnvironmentVariables(ctx context.Context, jobName string, req *DeleteEnvironmentVariablesRequest) error {
	if jobName == "" {
		return fmt.Errorf("jobName is required")
	}
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if req.ContainerName == "" {
		return fmt.Errorf("container_name is required")
	}
	if len(req.Env) == 0 {
		return fmt.Errorf("env array cannot be empty")
	}
	path := fmt.Sprintf("/job-deployments/%s/environment-variables", jobName)
	_, err := deleteRequestWithBody(ctx, s.client, path, req)
	return err
}
