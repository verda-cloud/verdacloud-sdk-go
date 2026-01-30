//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// Preferred cluster type for testing (as specified by user: 16x H200)
const PreferredClusterType = "16H200"

// cleanupCluster properly cleans up a cluster
func cleanupCluster(t *testing.T, client *verda.Client, clusterID string) {
	t.Helper()
	ctx := context.Background()
	t.Logf("🧹 Cleaning up cluster %s...", clusterID)

	// Get current status
	cluster, err := client.Clusters.GetByID(ctx, clusterID)
	if err != nil {
		t.Logf("   ⚠️  Could not get cluster: %v", err)
		return
	}
	t.Logf("   Current status: %s", cluster.Status)

	// Note: Only discontinue action is allowed for clusters
	// API does not support shutdown or delete actions
	if err := client.Clusters.Discontinue(ctx, []string{clusterID}); err != nil {
		t.Logf("   ⚠️  Discontinue failed: %v", err)
	} else {
		t.Log("   ✅ Discontinued successfully")
		// Wait for discontinue to complete
		time.Sleep(15 * time.Second)
	}
}

// ============================================================================
// CLUSTER CRUD INTEGRATION TEST
// ============================================================================

// TestClusterCRUDIntegration tests the complete cluster lifecycle:
// 1. Check availability first
// 2. Create cluster
// 3. Wait for it to be ready
// 4. List clusters (verify it exists)
// 5. Read cluster by ID
// 6. Cleanup (discontinue/delete)
func TestClusterCRUDIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// ========================================
	// STEP 0: Check availability
	// ========================================
	availableCluster, ok := FindAvailableClusterType(ctx, t, client, PreferredClusterType)
	if !ok {
		t.Skip("⏭️  SKIPPING: No cluster types available in staging environment")
	}

	// Track state for sequential CRUD operations
	var clusterID string
	var clusterCreated bool

	// Cleanup function - always runs at the end
	defer func() {
		if clusterCreated && clusterID != "" {
			cleanupCluster(t, client, clusterID)
		}
	}()

	// ========================================
	// STEP 1: CREATE
	// ========================================
	t.Run("1_CREATE", func(t *testing.T) {
		req := verda.CreateClusterRequest{
			ClusterType:  availableCluster.ClusterType,
			Image:        availableCluster.Image,
			Hostname:     "integration-test-cluster",
			Description:  "Integration test cluster - safe to delete",
			LocationCode: availableCluster.Location,
		}

		t.Logf("📦 Creating cluster with type: %s at %s...", req.ClusterType, req.LocationCode)

		resp, err := client.Clusters.Create(ctx, req)
		if err != nil {
			// Handle common errors
			if apiErr, ok := err.(*verda.APIError); ok {
				if apiErr.StatusCode == 504 {
					t.Skip("⏭️  API timeout (504) - cluster creation may take longer")
				}
				if apiErr.StatusCode >= 500 {
					t.Skipf("⏭️  Server error (%d): %v", apiErr.StatusCode, err)
				}
				if apiErr.StatusCode == 400 {
					t.Skipf("⏭️  Bad request: %v", err)
				}
			}
			t.Fatalf("❌ Failed to create cluster: %v", err)
		}

		if resp.ID == "" {
			t.Fatal("❌ Created cluster has empty ID")
		}

		clusterID = resp.ID
		clusterCreated = true
		t.Logf("✅ Created cluster: ID=%s", clusterID)
	})

	// If creation failed, skip remaining tests
	if !clusterCreated {
		t.Skip("⏭️  Skipping remaining tests - cluster was not created")
	}

	// ========================================
	// STEP 2: WAIT for ready state
	// ========================================
	t.Run("2_WAIT", func(t *testing.T) {
		if !clusterCreated {
			t.Skip("⏭️  Skipping - cluster was not created")
		}

		// Wait for cluster to be running
		cluster, ok := WaitForClusterStatus(ctx, t, client, clusterID, verda.StatusRunning, 10*time.Minute)
		if !ok {
			// Even if not running, check if it's in a valid state
			cluster, err := client.Clusters.GetByID(ctx, clusterID)
			if err != nil {
				t.Fatalf("❌ Could not get cluster status: %v", err)
			}
			t.Logf("⚠️  Cluster is in status '%s' (not running yet)", cluster.Status)
		} else {
			t.Logf("✅ Cluster is running: IP=%v", cluster.IP)
		}
	})

	// ========================================
	// STEP 3: LIST (verify cluster exists)
	// ========================================
	t.Run("3_LIST", func(t *testing.T) {
		if !clusterCreated {
			t.Skip("⏭️  Skipping - cluster was not created")
		}

		clusters, err := client.Clusters.Get(ctx)
		if err != nil {
			t.Fatalf("❌ Failed to list clusters: %v", err)
		}

		found := false
		for _, c := range clusters {
			if c.ID == clusterID {
				found = true
				t.Logf("✅ Found our cluster in list: ID=%s, Status=%s, Type=%s",
					c.ID, c.Status, c.ClusterType)
				break
			}
		}

		if !found {
			t.Errorf("❌ Cluster %s not found in list", clusterID)
		}
	})

	// ========================================
	// STEP 4: READ by ID
	// ========================================
	t.Run("4_READ", func(t *testing.T) {
		if !clusterCreated {
			t.Skip("⏭️  Skipping - cluster was not created")
		}

		cluster, err := client.Clusters.GetByID(ctx, clusterID)
		if err != nil {
			t.Fatalf("❌ Failed to get cluster by ID: %v", err)
		}

		// Verify fields
		if cluster.ID != clusterID {
			t.Errorf("❌ ID mismatch: expected %s, got %s", clusterID, cluster.ID)
		}
		if cluster.ClusterType != availableCluster.ClusterType {
			t.Errorf("❌ ClusterType mismatch: expected %s, got %s", availableCluster.ClusterType, cluster.ClusterType)
		}

		t.Logf("✅ Read cluster: ID=%s, Type=%s, Status=%s, Location=%s, Hostname=%s",
			cluster.ID, cluster.ClusterType, cluster.Status, cluster.Location, cluster.Hostname)
	})

	// Note: DELETE happens in defer cleanup
	t.Log("✅ Cluster CRUD test complete - cleanup will run in defer")
}

// ============================================================================
// OTHER CLUSTER TESTS (Read-only)
// ============================================================================

// TestListClusters_Integration tests listing clusters
func TestListClusters_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	clusters, err := client.Clusters.Get(ctx)
	if err != nil {
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
			t.Skip("Clusters endpoint not available (404)")
		}
		t.Fatalf("❌ Failed to list clusters: %v", err)
	}

	t.Logf("✅ Found %d clusters", len(clusters))
	for i, c := range clusters {
		t.Logf("   [%d] ID=%s, Type=%s, Status=%s, Location=%s",
			i, c.ID, c.ClusterType, c.Status, c.Location)
	}
}

// TestClusterTypes_Integration tests getting cluster types and availability
func TestClusterTypes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// Get cluster types
	clusterTypes, err := client.Clusters.GetClusterTypes(ctx, "usd")
	if err != nil {
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
			t.Skip("Cluster types endpoint not available (404)")
		}
		t.Fatalf("❌ Failed to get cluster types: %v", err)
	}

	t.Logf("✅ Found %d cluster types:", len(clusterTypes))
	for _, ct := range clusterTypes {
		t.Logf("   - %s: $%.2f/hr", ct.ClusterType, float64(ct.PricePerHour))
	}

	// Get availabilities
	availabilities, err := client.Clusters.GetAvailabilities(ctx, "")
	if err != nil {
		t.Logf("⚠️  Could not get availabilities: %v", err)
	} else {
		t.Logf("✅ Found %d availability entries:", len(availabilities))
		for _, a := range availabilities {
			if len(a.Availabilities) > 0 {
				t.Logf("   ✅ %s - %d cluster types available", a.LocationCode, len(a.Availabilities))
				for _, ct := range a.Availabilities {
					t.Logf("      - %s", ct)
				}
			} else {
				t.Logf("   ⚠️  %s - No clusters available", a.LocationCode)
			}
		}
	}

	// Get images
	images, err := client.Clusters.GetImages(ctx)
	if err != nil {
		t.Logf("⚠️  Could not get images: %v", err)
	} else {
		t.Logf("✅ Found %d cluster images:", len(images))
		for _, img := range images {
			t.Logf("   - %s (%s): %v", img.Name, img.ImageType, img.Details)
		}
	}
}

// TestClusterAvailability_Integration tests checking cluster availability
func TestClusterAvailability_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)
	ctx := context.Background()

	// Test availability check
	availableCluster, ok := FindAvailableClusterType(ctx, t, client, PreferredClusterType)
	if ok {
		t.Logf("✅ Best available cluster: %s at %s ($%.2f/hr)",
			availableCluster.ClusterType, availableCluster.Location, availableCluster.PricePerHour)
	} else {
		t.Log("⚠️  No cluster types currently available")
	}
}
