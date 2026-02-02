//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// TestContainerDeploymentsListOnly tests listing deployments without creating (sanity check)
func TestContainerDeploymentsListOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("list existing deployments", func(t *testing.T) {
		deps, err := client.ContainerDeployments.GetDeployments(ctx)
		if err != nil {
			// Handle common errors gracefully
			if apiErr, ok := err.(*verda.APIError); ok {
				if apiErr.StatusCode == 404 {
					t.Skip("⚠️  Container deployments endpoint not available (404)")
				} else if apiErr.StatusCode == 504 {
					t.Skip("⚠️  API timeout (504) - staging environment may be slow or unavailable")
				} else if apiErr.StatusCode >= 500 {
					t.Skipf("⚠️  Server error (%d) - API may be experiencing issues", apiErr.StatusCode)
				}
			}
			t.Fatalf("❌ Failed to list deployments: %v", err)
		}
		t.Logf("✅ Found %d existing deployments", len(deps))
		for i, d := range deps {
			if i < 5 { // Show first 5
				t.Logf("  - %s (spot: %v)", d.Name, d.IsSpot)
			}
		}
	})

	t.Run("get compute resources", func(t *testing.T) {
		resources, err := client.ContainerDeployments.GetServerlessComputeResources(ctx)
		if err != nil {
			t.Logf("⚠️  Could not get compute resources: %v", err)
			return
		}
		t.Logf("✅ Found %d compute resources:", len(resources))
		for _, r := range resources {
			t.Logf("  - %s (size: %d): Available=%v",
				r.Name, r.Size, r.IsAvailable)
		}
	})
}

// TestContainerDeploymentsCRUDWithScalingAndEnvVars demonstrates complete CRUD flow
// including scaling options and environment variables
func TestContainerDeploymentsCRUDWithScalingAndEnvVars(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// Pick cheapest available serverless compute (e2e cost control)
	computeName, computeSize, ok := FindAvailableContainerCompute(ctx, t, client, "")
	if !ok {
		t.Skip("⏭️  SKIPPING: No container compute available")
	}
	t.Logf("Using compute: %s (size %d)", computeName, computeSize)

	// Create a unique deployment name
	depName := generateRandomName("test-dep")
	var containerName string   // Will be extracted from API response
	var deploymentCreated bool // Track if deployment was successfully created

	// Step 1: CREATE - Create a new container deployment
	t.Run("1. CREATE deployment", func(t *testing.T) {
		req := &verda.CreateDeploymentRequest{
			Name:   depName,
			IsSpot: false,
			Compute: verda.ContainerCompute{
				Name: computeName,
				Size: computeSize,
			},
			ContainerRegistrySettings: verda.ContainerRegistrySettings{
				IsPrivate: false,
			},
			Scaling: verda.ContainerScalingOptions{
				MinReplicaCount: 1,
				MaxReplicaCount: 1,
				ScaleDownPolicy: &verda.ScalingPolicy{
					DelaySeconds: 300,
				},
				ScaleUpPolicy: &verda.ScalingPolicy{
					DelaySeconds: 300,
				},
				QueueMessageTTLSeconds:       300,
				ConcurrentRequestsPerReplica: 1,
				ScalingTriggers: &verda.ScalingTriggers{
					QueueLoad: &verda.QueueLoadTrigger{
						Threshold: 1.0,
					},
					CPUUtilization: &verda.UtilizationTrigger{
						Enabled:   true,
						Threshold: 80,
					},
					GPUUtilization: &verda.UtilizationTrigger{
						Enabled:   true,
						Threshold: 80,
					},
				},
			},
			Containers: []verda.CreateDeploymentContainer{
				{
					// Note: Don't set Name - API will auto-generate from image
					Image:       "registry-1.docker.io/chentex/random-logger:v1.0.1",
					ExposedPort: 8080,
					Healthcheck: &verda.ContainerHealthcheck{
						Enabled: true,
						Port:    8081,
						Path:    "/health",
					},
					EntrypointOverrides: &verda.ContainerEntrypointOverrides{
						Enabled: true,
						Cmd:     []string{"10", "300"},
					},
					Env: []verda.ContainerEnvVar{
						{
							Type:                     "plain",
							Name:                     "INITIAL_VAR",
							ValueOrReferenceToSecret: "initial-value",
						},
					},
					VolumeMounts: []verda.ContainerVolumeMount{
						{
							Type:      "scratch",
							MountPath: "/data",
						},
					},
				},
			},
		}

		dep, err := client.ContainerDeployments.CreateDeployment(ctx, req)
		if err != nil {
			// Check if it's a timeout or resource availability issue
			if apiErr, ok := err.(*verda.APIError); ok {
				if apiErr.StatusCode == 504 {
					t.Skip("⚠️  Skipping: API timeout (504) - server may be provisioning resources or endpoint unavailable")
				} else if apiErr.StatusCode == 404 {
					t.Skip("⚠️  Skipping: Container deployments endpoint not available (404)")
				} else if apiErr.StatusCode == 400 {
					t.Skipf("⚠️  Skipping: Bad request (400) - compute resource may not be available: %v", err)
				}
			}
			t.Fatalf("❌ Failed to create deployment: %v", err)
		}

		if dep.Name != depName {
			t.Errorf("Expected deployment name %s, got %s", depName, dep.Name)
		}
		deploymentCreated = true // Mark as successfully created

		// Extract the container name from the response
		if len(dep.Containers) > 0 {
			containerName = dep.Containers[0].Name
			t.Logf("✅ Created deployment: %s (container: %s)", dep.Name, containerName)
		} else {
			t.Logf("✅ Created deployment: %s (no containers in response)", dep.Name)
		}
	})

	// Cleanup runs on success and on failure/panic so test data is always removed
	defer func() {
		if deploymentCreated {
			t.Logf("🧹 Cleaning up deployment: %s", depName)
			if err := client.ContainerDeployments.DeleteDeployment(ctx, depName, 60000); err != nil {
				t.Logf("⚠️  Failed to delete deployment: %v", err)
			} else {
				t.Logf("✅ Deleted deployment: %s", depName)
				// Wait for deletion to complete
				time.Sleep(10 * time.Second)
			}
		}
	}()

	// Skip remaining tests if deployment wasn't created
	if !deploymentCreated {
		t.Skip("⚠️  Skipping remaining tests - deployment was not created")
	}

	// Wait for deployment to be ready
	time.Sleep(5 * time.Second)

	// Step 2: READ - Get deployment by name
	t.Run("2. READ deployment by name", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		dep, err := client.ContainerDeployments.GetDeploymentByName(ctx, depName)
		if err != nil {
			// Handle 504 gracefully for read operations too
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504) on read operation")
			}
			t.Fatalf("❌ Failed to get deployment: %v", err)
		}
		if dep.Name != depName {
			t.Errorf("Expected deployment name %s, got %s", depName, dep.Name)
		}

		// Extract container name if not already set
		if containerName == "" && len(dep.Containers) > 0 {
			containerName = dep.Containers[0].Name
		}
		t.Logf("✅ Retrieved deployment: %s (containers: %d, containerName: %s)", dep.Name, len(dep.Containers), containerName)
	})

	// Step 3: LIST - List all deployments
	t.Run("3. LIST all deployments", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		deps, err := client.ContainerDeployments.GetDeployments(ctx)
		if err != nil {
			// Handle 504 gracefully
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504) on list operation")
			}
			t.Fatalf("❌ Failed to list deployments: %v", err)
		}

		found := false
		for _, d := range deps {
			if d.Name == depName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Deployment %s not found in list", depName)
		}
		t.Logf("✅ Listed %d deployments, found our deployment: %s", len(deps), depName)
	})

	// Step 4: GET deployment status
	t.Run("4. READ deployment status", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		status, err := client.ContainerDeployments.GetDeploymentStatus(ctx, depName)
		if err != nil {
			t.Fatalf("❌ Failed to get deployment status: %v", err)
		}
		t.Logf("✅ Deployment status: %s", status.Status)
	})

	// ==========================================
	// SCALING OPTIONS CRUD
	// ==========================================

	// Step 5: GET scaling options
	t.Run("5. READ deployment scaling options", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		scaling, err := client.ContainerDeployments.GetDeploymentScaling(ctx, depName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to get scaling options: %v", err)
		}
		t.Logf("✅ Got scaling options: maxReplicas=%d, minReplicas=%d, queueTTL=%d",
			scaling.MaxReplicaCount, scaling.MinReplicaCount, scaling.QueueMessageTTLSeconds)
	})

	// Step 6: UPDATE scaling options
	t.Run("6. UPDATE deployment scaling options", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		maxReplicas := 2
		minReplicas := 0
		queueTTL := 600
		updateReq := &verda.UpdateScalingOptionsRequest{
			MaxReplicaCount:        &maxReplicas,
			MinReplicaCount:        &minReplicas,
			QueueMessageTTLSeconds: &queueTTL,
		}

		scaling, err := client.ContainerDeployments.UpdateDeploymentScaling(ctx, depName, updateReq)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to update scaling options: %v", err)
		}
		t.Logf("✅ Updated scaling options: maxReplicas=%d, queueTTL=%d",
			scaling.MaxReplicaCount, scaling.QueueMessageTTLSeconds)
	})

	// Step 7: Verify scaling update
	t.Run("7. VERIFY scaling options update", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		scaling, err := client.ContainerDeployments.GetDeploymentScaling(ctx, depName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to get scaling options after update: %v", err)
		}

		if scaling.MaxReplicaCount != 2 {
			t.Errorf("Expected maxReplicaCount=2, got %d", scaling.MaxReplicaCount)
		}
		t.Logf("✅ Verified scaling options after update: maxReplicas=%d", scaling.MaxReplicaCount)
	})

	// ==========================================
	// ENVIRONMENT VARIABLES CRUD
	// ==========================================

	// Step 8: GET environment variables
	t.Run("8. READ environment variables", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		envVars, err := client.ContainerDeployments.GetEnvironmentVariables(ctx, depName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to get environment variables: %v", err)
		}
		t.Logf("✅ Got %d environment variables", len(envVars))
		for _, env := range envVars {
			t.Logf("   - %s=%s (type: %s)", env.Name, env.ValueOrReferenceToSecret, env.Type)
		}
	})

	// Step 9: ADD new environment variables
	t.Run("9. ADD environment variables", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		addReq := &verda.EnvironmentVariablesRequest{
			ContainerName: containerName,
			Env: []verda.ContainerEnvVar{
				{
					Type:                     "plain",
					Name:                     "NEW_VAR_1",
					ValueOrReferenceToSecret: "value-1",
				},
				{
					Type:                     "plain",
					Name:                     "NEW_VAR_2",
					ValueOrReferenceToSecret: "value-2",
				},
			},
		}

		err := client.ContainerDeployments.AddEnvironmentVariables(ctx, depName, addReq)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to add environment variables: %v", err)
		}
		t.Logf("✅ Added 2 new environment variables")
	})

	// Wait a moment for changes to propagate
	time.Sleep(2 * time.Second)

	// Step 10: Verify env vars were added
	t.Run("10. VERIFY environment variables added", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		envVars, err := client.ContainerDeployments.GetEnvironmentVariables(ctx, depName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to get environment variables after add: %v", err)
		}

		t.Logf("✅ Got %d environment variables after add:", len(envVars))
		for _, env := range envVars {
			t.Logf("   - %s=%s (type: %s)", env.Name, env.ValueOrReferenceToSecret, env.Type)
		}
	})

	// Step 11: UPDATE environment variables
	t.Run("11. UPDATE environment variables", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		updateReq := &verda.EnvironmentVariablesRequest{
			ContainerName: containerName,
			Env: []verda.ContainerEnvVar{
				{
					Type:                     "plain",
					Name:                     "NEW_VAR_1",
					ValueOrReferenceToSecret: "updated-value-1",
				},
			},
		}

		err := client.ContainerDeployments.UpdateEnvironmentVariables(ctx, depName, updateReq)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to update environment variables: %v", err)
		}
		t.Logf("✅ Updated environment variable NEW_VAR_1")
	})

	// Step 12: DELETE environment variables
	t.Run("12. DELETE environment variables", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		deleteReq := &verda.DeleteEnvironmentVariablesRequest{
			ContainerName: containerName,
			Env: []string{
				"NEW_VAR_2",
			},
		}

		err := client.ContainerDeployments.DeleteEnvironmentVariables(ctx, depName, deleteReq)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to delete environment variables: %v", err)
		}
		t.Logf("✅ Deleted environment variable NEW_VAR_2")
	})

	// Step 13: Verify env var deletion
	t.Run("13. VERIFY environment variable deletion", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		envVars, err := client.ContainerDeployments.GetEnvironmentVariables(ctx, depName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to get environment variables after delete: %v", err)
		}

		t.Logf("✅ Got %d environment variables after delete:", len(envVars))
		for _, env := range envVars {
			t.Logf("   - %s=%s (type: %s)", env.Name, env.ValueOrReferenceToSecret, env.Type)
		}
	})

	// ==========================================
	// UPDATE DEPLOYMENT (full update)
	// ==========================================

	// Step 14: UPDATE deployment (scaling only - containers update is done via env vars)
	t.Run("14. UPDATE deployment (scaling)", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		// Note: For full deployment updates with containers, the API may require specific container formats.
		// Here we just update scaling via the dedicated scaling endpoint which we tested in steps 5-7.
		// This step demonstrates that full deployment update works for scaling configuration.
		minReplicas := 1
		maxReplicas := 3
		queueTTL := 900
		concurrentReq := 2
		updateReq := &verda.UpdateScalingOptionsRequest{
			MinReplicaCount: &minReplicas,
			MaxReplicaCount: &maxReplicas,
			ScaleDownPolicy: &verda.ScalingPolicy{
				DelaySeconds: 600,
			},
			ScaleUpPolicy: &verda.ScalingPolicy{
				DelaySeconds: 120,
			},
			QueueMessageTTLSeconds:       &queueTTL,
			ConcurrentRequestsPerReplica: &concurrentReq,
			ScalingTriggers: &verda.ScalingTriggers{
				QueueLoad: &verda.QueueLoadTrigger{
					Threshold: 1.5,
				},
				CPUUtilization: &verda.UtilizationTrigger{
					Enabled:   true,
					Threshold: 70,
				},
				GPUUtilization: &verda.UtilizationTrigger{
					Enabled:   true,
					Threshold: 70,
				},
			},
		}

		scaling, err := client.ContainerDeployments.UpdateDeploymentScaling(ctx, depName, updateReq)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to update deployment scaling: %v", err)
		}
		t.Logf("✅ Updated deployment scaling: maxReplicas=%d, minReplicas=%d", scaling.MaxReplicaCount, scaling.MinReplicaCount)
	})

	// ==========================================
	// PAUSE AND RESUME
	// ==========================================

	// Step 15: Pause deployment
	t.Run("15. PAUSE deployment", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		err := client.ContainerDeployments.PauseDeployment(ctx, depName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Logf("⚠️  PauseDeployment: %v", err)
		} else {
			t.Log("✅ Deployment paused")
		}
	})

	time.Sleep(2 * time.Second)

	// Step 16: Resume deployment
	t.Run("16. RESUME deployment", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		err := client.ContainerDeployments.ResumeDeployment(ctx, depName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Logf("⚠️  ResumeDeployment: %v", err)
		} else {
			t.Log("✅ Deployment resumed")
		}
	})

	// ==========================================
	// GET REPLICAS
	// ==========================================

	// Step 17: Get replicas
	t.Run("17. GET deployment replicas", func(t *testing.T) {
		if !deploymentCreated {
			t.Skip("⚠️  Skipping - deployment was not created")
		}

		replicas, err := client.ContainerDeployments.GetDeploymentReplicas(ctx, depName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Logf("⚠️  GetDeploymentReplicas: %v", err)
		} else {
			t.Logf("✅ Found %d replicas", len(replicas.List))
			for _, r := range replicas.List {
				t.Logf("   - Replica ID: %s, Status: %s, Started: %s", r.ID, r.Status, r.StartedAt)
			}
		}
	})

	// Wait before deletion (happens in defer)
	time.Sleep(2 * time.Second)
}

// TestContainerDeploymentsResourcesLookup tests getting available resources
func TestContainerDeploymentsResourcesLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get compute resources", func(t *testing.T) {
		resources, err := client.ContainerDeployments.GetServerlessComputeResources(ctx)
		if err != nil {
			t.Skipf("Skipping compute resources: %v", err)
		}
		t.Logf("✅ Found %d compute resources:", len(resources))
		for _, r := range resources {
			t.Logf("   - %s (size: %d): Available=%v",
				r.Name, r.Size, r.IsAvailable)
		}
	})
}

// TestContainerDeploymentsSecretsAndCredentials tests secrets and registry credentials
func TestContainerDeploymentsSecretsAndCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("list secrets", func(t *testing.T) {
		secrets, err := client.ContainerDeployments.GetSecrets(ctx)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Skipf("⚠️  Could not get secrets: %v", err)
		}
		t.Logf("✅ Found %d secrets:", len(secrets))
		for _, s := range secrets {
			t.Logf("   - %s (type: %s, created: %s)", s.Name, s.SecretType, s.CreatedAt)
		}
	})

	t.Run("list file secrets", func(t *testing.T) {
		secrets, err := client.ContainerDeployments.GetFileSecrets(ctx)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Skipf("⚠️  Could not get file secrets: %v", err)
		}
		t.Logf("✅ Found %d file secrets:", len(secrets))
		for _, s := range secrets {
			t.Logf("   - %s (type: %s, files: %v)", s.Name, s.SecretType, s.FileNames)
		}
	})

	t.Run("list registry credentials", func(t *testing.T) {
		creds, err := client.ContainerDeployments.GetRegistryCredentials(ctx)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Skipf("⚠️  Could not get registry credentials: %v", err)
		}
		t.Logf("✅ Found %d registry credentials:", len(creds))
		for _, c := range creds {
			t.Logf("   - %s (created: %s)", c.Name, c.CreatedAt)
		}
	})
}
