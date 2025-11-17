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
	job, _, err := postRequest[JobDeployment](ctx, s.client, "/job-deployments", req)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *ServerlessJobsService) GetJobDeploymentByName(ctx context.Context, jobName string) (*JobDeployment, error) {
	path := fmt.Sprintf("/job-deployments/%s", jobName)
	job, _, err := getRequest[JobDeployment](ctx, s.client, path)
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
	path := fmt.Sprintf("/job-deployments/%s/scaling", jobName)
	scaling, _, err := getRequest[JobScalingOptions](ctx, s.client, path)
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
