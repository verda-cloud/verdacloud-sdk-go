package verda

import (
	"context"
	"fmt"
	"net/url"
)

type ContainerDeploymentsService struct {
	client *Client
}

func (s *ContainerDeploymentsService) GetDeployments(ctx context.Context) ([]ContainerDeployment, error) {
	deployments, _, err := getRequest[[]ContainerDeployment](ctx, s.client, "/container-deployments")
	if err != nil {
		return nil, err
	}
	return deployments, nil
}

func (s *ContainerDeploymentsService) CreateDeployment(ctx context.Context, req *CreateDeploymentRequest) (*ContainerDeployment, error) {
	deployment, _, err := postRequest[ContainerDeployment](ctx, s.client, "/container-deployments", req)
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

func (s *ContainerDeploymentsService) GetDeploymentByName(ctx context.Context, deploymentName string) (*ContainerDeployment, error) {
	path := fmt.Sprintf("/container-deployments/%s", deploymentName)
	deployment, _, err := getRequest[ContainerDeployment](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

func (s *ContainerDeploymentsService) UpdateDeployment(ctx context.Context, deploymentName string, req *UpdateDeploymentRequest) (*ContainerDeployment, error) {
	path := fmt.Sprintf("/container-deployments/%s", deploymentName)
	deployment, _, err := patchRequest[ContainerDeployment](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

// DeleteDeployment removes a deployment with optional timeout (0-300000ms, default 60000)
func (s *ContainerDeploymentsService) DeleteDeployment(ctx context.Context, deploymentName string, timeoutMs int) error {
	path := fmt.Sprintf("/container-deployments/%s", deploymentName)

	if timeoutMs > 0 {
		params := url.Values{}
		params.Set("timeout", fmt.Sprintf("%d", timeoutMs))
		path += "?" + params.Encode()
	}

	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}

func (s *ContainerDeploymentsService) GetDeploymentStatus(ctx context.Context, deploymentName string) (*DeploymentStatus, error) {
	path := fmt.Sprintf("/container-deployments/%s/status", deploymentName)
	status, _, err := getRequest[DeploymentStatus](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *ContainerDeploymentsService) RestartDeployment(ctx context.Context, deploymentName string) error {
	path := fmt.Sprintf("/container-deployments/%s/restart", deploymentName)
	_, _, err := postRequest[interface{}](ctx, s.client, path, nil)
	return err
}

func (s *ContainerDeploymentsService) PauseDeployment(ctx context.Context, deploymentName string) error {
	path := fmt.Sprintf("/container-deployments/%s/pause", deploymentName)
	_, _, err := postRequest[interface{}](ctx, s.client, path, nil)
	return err
}

func (s *ContainerDeploymentsService) ResumeDeployment(ctx context.Context, deploymentName string) error {
	path := fmt.Sprintf("/container-deployments/%s/resume", deploymentName)
	_, _, err := postRequest[interface{}](ctx, s.client, path, nil)
	return err
}

func (s *ContainerDeploymentsService) PurgeDeploymentQueue(ctx context.Context, deploymentName string) error {
	path := fmt.Sprintf("/container-deployments/%s/purge-queue", deploymentName)
	_, _, err := postRequest[interface{}](ctx, s.client, path, nil)
	return err
}

func (s *ContainerDeploymentsService) GetDeploymentScaling(ctx context.Context, deploymentName string) (*ScalingOptions, error) {
	path := fmt.Sprintf("/container-deployments/%s/scaling", deploymentName)
	scaling, _, err := getRequest[ScalingOptions](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &scaling, nil
}

func (s *ContainerDeploymentsService) UpdateDeploymentScaling(ctx context.Context, deploymentName string, req *UpdateScalingOptionsRequest) (*ScalingOptions, error) {
	path := fmt.Sprintf("/container-deployments/%s/scaling", deploymentName)
	scaling, _, err := patchRequest[ScalingOptions](ctx, s.client, path, req)
	if err != nil {
		return nil, err
	}
	return &scaling, nil
}

func (s *ContainerDeploymentsService) GetDeploymentReplicas(ctx context.Context, deploymentName string) (*DeploymentReplicas, error) {
	path := fmt.Sprintf("/container-deployments/%s/replicas", deploymentName)
	replicas, _, err := getRequest[DeploymentReplicas](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &replicas, nil
}

func (s *ContainerDeploymentsService) GetEnvironmentVariables(ctx context.Context, deploymentName string) (map[string]string, error) {
	path := fmt.Sprintf("/container-deployments/%s/environment-variables", deploymentName)
	envVars, _, err := getRequest[map[string]string](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return envVars, nil
}

func (s *ContainerDeploymentsService) AddEnvironmentVariables(ctx context.Context, deploymentName string, envVars map[string]string) error {
	path := fmt.Sprintf("/container-deployments/%s/environment-variables", deploymentName)
	req := &EnvironmentVariablesRequest{Variables: envVars}
	_, _, err := postRequest[interface{}](ctx, s.client, path, req)
	return err
}

func (s *ContainerDeploymentsService) UpdateEnvironmentVariables(ctx context.Context, deploymentName string, envVars map[string]string) error {
	path := fmt.Sprintf("/container-deployments/%s/environment-variables", deploymentName)
	req := &EnvironmentVariablesRequest{Variables: envVars}
	_, _, err := patchRequest[interface{}](ctx, s.client, path, req)
	return err
}

func (s *ContainerDeploymentsService) DeleteEnvironmentVariables(ctx context.Context, deploymentName string, varNames []string) error {
	path := fmt.Sprintf("/container-deployments/%s/environment-variables", deploymentName)
	req := &DeleteEnvironmentVariablesRequest{Names: varNames}
	_, err := deleteRequestWithBody(ctx, s.client, path, req)
	return err
}

func (s *ContainerDeploymentsService) GetServerlessComputeResources(ctx context.Context) ([]ComputeResource, error) {
	resources, _, err := getRequest[[]ComputeResource](ctx, s.client, "/serverless-compute-resources")
	if err != nil {
		return nil, err
	}
	return resources, nil
}

func (s *ContainerDeploymentsService) GetSecrets(ctx context.Context) ([]Secret, error) {
	secrets, _, err := getRequest[[]Secret](ctx, s.client, "/secrets")
	if err != nil {
		return nil, err
	}
	return secrets, nil
}

func (s *ContainerDeploymentsService) CreateSecret(ctx context.Context, req *CreateSecretRequest) error {
	_, _, err := postRequest[interface{}](ctx, s.client, "/secrets", req)
	return err
}

// DeleteSecret removes a secret - force deletes even if in use (dangerous)
func (s *ContainerDeploymentsService) DeleteSecret(ctx context.Context, secretName string, force bool) error {
	path := fmt.Sprintf("/secrets/%s", secretName)

	if force {
		params := url.Values{}
		params.Set("force", "true")
		path += "?" + params.Encode()
	}

	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}

func (s *ContainerDeploymentsService) GetFileSecrets(ctx context.Context) ([]FileSecret, error) {
	secrets, _, err := getRequest[[]FileSecret](ctx, s.client, "/file-secrets")
	if err != nil {
		return nil, err
	}
	return secrets, nil
}

func (s *ContainerDeploymentsService) CreateFileSecret(ctx context.Context, req *CreateFileSecretRequest) error {
	_, _, err := postRequest[interface{}](ctx, s.client, "/file-secrets", req)
	return err
}

func (s *ContainerDeploymentsService) DeleteFileSecret(ctx context.Context, secretName string, force bool) error {
	path := fmt.Sprintf("/file-secrets/%s", secretName)

	if force {
		params := url.Values{}
		params.Set("force", "true")
		path += "?" + params.Encode()
	}

	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}

func (s *ContainerDeploymentsService) GetRegistryCredentials(ctx context.Context) ([]RegistryCredentials, error) {
	credentials, _, err := getRequest[[]RegistryCredentials](ctx, s.client, "/container-registry-credentials")
	if err != nil {
		return nil, err
	}
	return credentials, nil
}

func (s *ContainerDeploymentsService) CreateRegistryCredentials(ctx context.Context, req *CreateRegistryCredentialsRequest) error {
	_, _, err := postRequest[interface{}](ctx, s.client, "/container-registry-credentials", req)
	return err
}

func (s *ContainerDeploymentsService) DeleteRegistryCredentials(ctx context.Context, credentialsName string, force bool) error {
	path := fmt.Sprintf("/container-registry-credentials/%s", credentialsName)

	if force {
		params := url.Values{}
		params.Set("force", "true")
		path += "?" + params.Encode()
	}

	_, err := deleteRequestNoResult(ctx, s.client, path)
	return err
}
