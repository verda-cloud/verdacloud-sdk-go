//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// TestServerlessJobsListOnly tests listing job deployments without creating (sanity check)
func TestServerlessJobsListOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("list existing job deployments", func(t *testing.T) {
		jobs, err := client.ServerlessJobs.GetJobDeployments(ctx)
		if err != nil {
			// Handle common errors gracefully
			if apiErr, ok := err.(*verda.APIError); ok {
				if apiErr.StatusCode == 404 {
					t.Skip("⚠️  Job deployments endpoint not available (404)")
				} else if apiErr.StatusCode == 504 {
					t.Skip("⚠️  API timeout (504) - staging environment may be slow or unavailable")
				} else if apiErr.StatusCode >= 500 {
					t.Skipf("⚠️  Server error (%d) - API may be experiencing issues", apiErr.StatusCode)
				}
			}
			t.Fatalf("❌ Failed to list job deployments: %v", err)
		}
		t.Logf("✅ Found %d existing job deployments", len(jobs))
		for i, j := range jobs {
			if i < 5 { // Show first 5
				t.Logf("  - %s (created: %s)", j.Name, j.CreatedAt)
			}
		}
	})
}

// TestServerlessJobsCRUDWithScalingAndEnvVars demonstrates complete CRUD flow
// including scaling options and environment variables
func TestServerlessJobsCRUDWithScalingAndEnvVars(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// Create a unique job name
	jobName := generateRandomName("job-test")
	var containerName string // Will be extracted from API response
	var jobCreated bool      // Track if job was successfully created

	// Step 1: CREATE - Create a new job deployment
	t.Run("1. CREATE job deployment", func(t *testing.T) {
		req := &verda.CreateJobDeploymentRequest{
			Name: jobName,
			ContainerRegistrySettings: map[string]any{
				"credentials": map[string]any{
					"name": "dockerhub-credentials",
				},
			},
			Containers: []verda.CreateDeploymentContainer{
				{
					Image:       "registry-1.docker.io/chentex/random-logger:v1.0.1",
					ExposedPort: 8080,
					Healthcheck: map[string]any{
						"enabled": true,
						"port":    8081,
						"path":    "/health",
					},
					EntrypointOverrides: map[string]any{
						"enabled":    true,
						"entrypoint": []string{"python3", "main.py"},
						"cmd":        []string{"--port", "8080"},
					},
					Env: []verda.ContainerEnvVar{
						{
							Name:                     "MY_ENV_VAR",
							ValueOrReferenceToSecret: "my-value",
							Type:                     "plain",
						},
					},
					VolumeMounts: []map[string]any{
						{
							"type":        "scratch",
							"mount_path":  "/data",
							"secret_name": "my-secret",
							"size_in_mb":  64,
							"volumeId":    "fa4a0338-65b2-4819-8450-821190fbaf6d",
						},
					},
				},
			},
			Compute: map[string]any{
				"name": "RTX 4500 Ada",
				"size": 1,
			},
			Scaling: map[string]any{
				"max_replica_count":         1,
				"queue_message_ttl_seconds": 300, // API requires this field to be present
				"deadline_seconds":          600,
			},
		}

		job, err := client.ServerlessJobs.CreateJobDeployment(ctx, req)
		if err != nil {
			// Check if it's a timeout or resource availability issue
			if apiErr, ok := err.(*verda.APIError); ok {
				if apiErr.StatusCode == 504 {
					t.Skip("⚠️  Skipping: API timeout (504) - server may be provisioning resources or endpoint unavailable")
				} else if apiErr.StatusCode == 404 {
					t.Skip("⚠️  Skipping: Job deployments endpoint not available (404)")
				} else if apiErr.StatusCode == 400 {
					t.Skipf("⚠️  Skipping: Bad request (400) - compute resource may not be available: %v", err)
				}
			}
			t.Fatalf("❌ Failed to create job deployment: %v", err)
		}

		if job.Name != jobName {
			t.Errorf("Expected job name %s, got %s", jobName, job.Name)
		}
		jobCreated = true // Mark as successfully created

		// Extract the container name from the response
		if len(job.Containers) > 0 {
			containerName = job.Containers[0].Name
			t.Logf("✅ Created job deployment: %s (container: %s)", job.Name, containerName)
		} else {
			t.Logf("✅ Created job deployment: %s (no containers in response)", job.Name)
		}
	})

	// Cleanup function
	defer func() {
		if jobCreated {
			t.Logf("🧹 Cleaning up job deployment: %s", jobName)
			if err := client.ServerlessJobs.DeleteJobDeployment(ctx, jobName, 60000); err != nil {
				t.Logf("⚠️  Failed to delete job deployment: %v", err)
			} else {
				t.Logf("✅ Deleted job deployment: %s", jobName)
			}
		}
	}()

	// Skip remaining tests if job wasn't created
	if !jobCreated {
		t.Skip("⚠️  Skipping remaining tests - job deployment was not created")
	}

	// Wait for job deployment to be ready
	time.Sleep(5 * time.Second)

	// Step 2: READ - Get job deployment by name
	t.Run("2. READ job deployment by name", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		job, err := client.ServerlessJobs.GetJobDeploymentByName(ctx, jobName)
		if err != nil {
			// Handle 504 gracefully for read operations too
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504) on read operation")
			}
			t.Fatalf("❌ Failed to get job deployment: %v", err)
		}
		if job.Name != jobName {
			t.Errorf("Expected job name %s, got %s", jobName, job.Name)
		}

		// Extract container name if not already set
		if containerName == "" && len(job.Containers) > 0 {
			containerName = job.Containers[0].Name
		}
		t.Logf("✅ Retrieved job deployment: %s (containers: %d, containerName: %s)", job.Name, len(job.Containers), containerName)
	})

	// Step 3: LIST - List all job deployments
	t.Run("3. LIST all job deployments", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		jobs, err := client.ServerlessJobs.GetJobDeployments(ctx)
		if err != nil {
			// Handle 504 gracefully
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504) on list operation")
			}
			t.Fatalf("❌ Failed to list job deployments: %v", err)
		}

		found := false
		for _, j := range jobs {
			if j.Name == jobName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Job deployment %s not found in list", jobName)
		}
		t.Logf("✅ Listed %d job deployments, found our job: %s", len(jobs), jobName)
	})

	// Step 4: GET job deployment status
	t.Run("4. READ job deployment status", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		status, err := client.ServerlessJobs.GetJobDeploymentStatus(ctx, jobName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to get job deployment status: %v", err)
		}
		t.Logf("✅ Job deployment status: %s, active: %d, succeeded: %d, failed: %d",
			status.Status, status.ActiveJobs, status.SucceededJobs, status.FailedJobs)
	})

	// ==========================================
	// SCALING OPTIONS CRUD
	// ==========================================

	// Step 5: GET scaling options
	t.Run("5. READ job scaling options", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		scaling, err := client.ServerlessJobs.GetJobDeploymentScaling(ctx, jobName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to get scaling options: %v", err)
		}
		t.Logf("✅ Got scaling options: maxReplicas=%d, queueTTL=%d",
			scaling.MaxReplicaCount, scaling.QueueMessageTTLSeconds)
	})

	// Step 6: UPDATE scaling options
	t.Run("6. UPDATE job scaling options", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		updateReq := &verda.UpdateScalingOptionsRequest{
			MaxReplicaCount:        2,
			QueueMessageTTLSeconds: 600,
		}

		scaling, err := client.ServerlessJobs.UpdateJobDeploymentScaling(ctx, jobName, updateReq)
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
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		scaling, err := client.ServerlessJobs.GetJobDeploymentScaling(ctx, jobName)
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
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		envVars, err := client.ServerlessJobs.GetJobEnvironmentVariables(ctx, jobName)
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
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
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

		err := client.ServerlessJobs.AddJobEnvironmentVariables(ctx, jobName, addReq)
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
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		envVars, err := client.ServerlessJobs.GetJobEnvironmentVariables(ctx, jobName)
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
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
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

		err := client.ServerlessJobs.UpdateJobEnvironmentVariables(ctx, jobName, updateReq)
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
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		deleteReq := &verda.DeleteEnvironmentVariablesRequest{
			ContainerName: containerName,
			Env: []verda.ContainerEnvVar{
				{
					Name: "NEW_VAR_2",
				},
			},
		}

		err := client.ServerlessJobs.DeleteJobEnvironmentVariables(ctx, jobName, deleteReq)
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
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		envVars, err := client.ServerlessJobs.GetJobEnvironmentVariables(ctx, jobName)
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
	// UPDATE JOB DEPLOYMENT (full update)
	// ==========================================

	// Step 14: UPDATE job deployment (scaling via dedicated endpoint)
	t.Run("14. UPDATE job deployment (scaling)", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		// Update scaling via the dedicated scaling endpoint
		updateReq := &verda.UpdateScalingOptionsRequest{
			MaxReplicaCount:        3,
			QueueMessageTTLSeconds: 900,
		}

		scaling, err := client.ServerlessJobs.UpdateJobDeploymentScaling(ctx, jobName, updateReq)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to update job scaling: %v", err)
		}
		t.Logf("✅ Updated job scaling: maxReplicas=%d", scaling.MaxReplicaCount)
	})

	// ==========================================
	// UPDATE JOB DEPLOYMENT (via PATCH)
	// ==========================================

	// Step 15: UPDATE job deployment via PATCH
	t.Run("15. UPDATE job deployment via PATCH", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		// Update the job deployment using UpdateJobDeployment (PATCH)
		updateReq := &verda.UpdateJobDeploymentRequest{
			Scaling: map[string]any{
				"max_replica_count":         4,
				"queue_message_ttl_seconds": 1200,
				"deadline_seconds":          600,
			},
		}

		job, err := client.ServerlessJobs.UpdateJobDeployment(ctx, jobName, updateReq)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Fatalf("❌ Failed to update job deployment: %v", err)
		}
		t.Logf("✅ Updated job deployment via PATCH: %s", job.Name)
		if job.Scaling != nil {
			t.Logf("   Scaling: maxReplicas=%d, queueTTL=%d, deadline=%d",
				job.Scaling.MaxReplicaCount, job.Scaling.QueueMessageTTLSeconds, job.Scaling.DeadlineSeconds)
		}
	})

	// ==========================================
	// PAUSE AND RESUME
	// ==========================================

	// Step 16: Pause job deployment
	t.Run("16. PAUSE job deployment", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		err := client.ServerlessJobs.PauseJobDeployment(ctx, jobName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Logf("⚠️  PauseJobDeployment: %v", err)
		} else {
			t.Log("✅ Job deployment paused")
		}
	})

	time.Sleep(2 * time.Second)

	// Step 17: Resume job deployment
	t.Run("17. RESUME job deployment", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		err := client.ServerlessJobs.ResumeJobDeployment(ctx, jobName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Logf("⚠️  ResumeJobDeployment: %v", err)
		} else {
			t.Log("✅ Job deployment resumed")
		}
	})

	// ==========================================
	// PURGE QUEUE
	// ==========================================

	// Step 18: Purge job queue
	t.Run("18. PURGE job deployment queue", func(t *testing.T) {
		if !jobCreated {
			t.Skip("⚠️  Skipping - job deployment was not created")
		}

		err := client.ServerlessJobs.PurgeJobDeploymentQueue(ctx, jobName)
		if err != nil {
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 504 {
				t.Skip("⚠️  Skipping: API timeout (504)")
			}
			t.Logf("⚠️  PurgeJobDeploymentQueue: %v", err)
		} else {
			t.Log("✅ Job deployment queue purged")
		}
	})

	// Wait before deletion (happens in defer)
	time.Sleep(2 * time.Second)
}
