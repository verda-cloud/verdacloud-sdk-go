//go:build integration
// +build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

// getTestClient creates a client for integration tests using environment variables
func getTestClient(t *testing.T) *verda.Client {
	t.Helper()

	// Get credentials and configuration from environment
	clientID := os.Getenv("VERDA_CLIENT_ID")
	clientSecret := os.Getenv("VERDA_CLIENT_SECRET")
	baseURL := os.Getenv("VERDA_BASE_URL") // Optional: defaults to production

	if clientID == "" || clientSecret == "" {
		t.Skip("VERDA_CLIENT_ID and VERDA_CLIENT_SECRET must be set for integration tests")
	}

	// Create client with optional base URL override
	options := []verda.ClientOption{
		verda.WithClientID(clientID),
		verda.WithClientSecret(clientSecret),
	}
	if baseURL != "" {
		options = append(options, verda.WithBaseURL(baseURL))
		t.Logf("Using custom base URL: %s", baseURL)
	}

	client, err := verda.NewClient(options...)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return client
}

func generateRandomName(prefix string) string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	if err != nil {
		return prefix + "-test"
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// ============================================================================
// AVAILABILITY CHECKING HELPERS
// ============================================================================

// AvailableInstanceType holds info about an available instance type
type AvailableInstanceType struct {
	InstanceType string
	SpotPrice    float64
	Location     string
}

// AvailableClusterType holds info about an available cluster type
type AvailableClusterType struct {
	ClusterType  string
	PricePerHour float64
	Location     string
	Image        string
}

// FindAvailableInstanceType checks availability and returns the best instance type to use
// It prefers the preferredType if available, otherwise returns the cheapest available
func FindAvailableInstanceType(ctx context.Context, t *testing.T, client *verda.Client, preferredType string) (*AvailableInstanceType, bool) {
	t.Helper()

	t.Log("🔍 Checking instance type availability...")

	// Get all instance types with pricing
	instanceTypes, err := client.InstanceTypes.Get(ctx, "usd")
	if err != nil {
		t.Logf("⚠️  Could not get instance types: %v", err)
		return nil, false
	}
	t.Logf("   Found %d instance types", len(instanceTypes))

	// Check if preferred type is available
	if preferredType != "" {
		available, err := client.Instances.CheckInstanceTypeAvailability(ctx, preferredType)
		if err == nil && available {
			// Find price info
			var price float64
			for _, it := range instanceTypes {
				if it.InstanceType == preferredType {
					price = float64(it.SpotPrice)
					break
				}
			}
			t.Logf("✅ Preferred instance type %s is AVAILABLE ($%.2f/hr spot)", preferredType, price)
			return &AvailableInstanceType{
				InstanceType: preferredType,
				SpotPrice:    price,
				Location:     verda.LocationFIN03,
			}, true
		}
		t.Logf("⚠️  Preferred instance type %s is NOT available", preferredType)
	}

	// Sort instance types by spot price (cheapest first)
	sort.Slice(instanceTypes, func(i, j int) bool {
		return float64(instanceTypes[i].SpotPrice) < float64(instanceTypes[j].SpotPrice)
	})

	// Find the cheapest available instance type
	for _, it := range instanceTypes {
		available, err := client.Instances.CheckInstanceTypeAvailability(ctx, it.InstanceType)
		if err == nil && available {
			t.Logf("✅ Found cheapest available: %s ($%.2f/hr spot)", it.InstanceType, float64(it.SpotPrice))
			return &AvailableInstanceType{
				InstanceType: it.InstanceType,
				SpotPrice:    float64(it.SpotPrice),
				Location:     verda.LocationFIN03,
			}, true
		}
	}

	t.Log("❌ No instance types available in staging environment")
	return nil, false
}

// FindAvailableClusterType checks availability and returns the best cluster type to use
func FindAvailableClusterType(ctx context.Context, t *testing.T, client *verda.Client, preferredType string) (*AvailableClusterType, bool) {
	t.Helper()

	t.Log("🔍 Checking cluster type availability...")

	// Get all cluster types with pricing
	clusterTypes, err := client.Clusters.GetClusterTypes(ctx, "usd")
	if err != nil {
		t.Logf("⚠️  Could not get cluster types: %v", err)
		return nil, false
	}
	t.Logf("   Found %d cluster types", len(clusterTypes))

	// Get all availabilities
	availabilities, err := client.Clusters.GetAvailabilities(ctx, "")
	if err != nil {
		t.Logf("⚠️  Could not get cluster availabilities: %v", err)
		return nil, false
	}

	// Build a map of available cluster types by location
	availableMap := make(map[string]map[string]bool) // clusterType -> locationCode -> available
	for _, a := range availabilities {
		// Each availability entry has location_code and list of available cluster types
		for _, clusterType := range a.Availabilities {
			if _, ok := availableMap[clusterType]; !ok {
				availableMap[clusterType] = make(map[string]bool)
			}
			availableMap[clusterType][a.LocationCode] = true
		}
	}

	// Get cluster images
	images, err := client.Clusters.GetImages(ctx)
	if err != nil {
		t.Logf("⚠️  Could not get cluster images: %v", err)
		return nil, false
	}
	var defaultImage string
	if len(images) > 0 {
		defaultImage = images[0].Name
	}

	// Check if preferred type is available
	if preferredType != "" {
		if locs, ok := availableMap[preferredType]; ok {
			for loc, avail := range locs {
				if avail {
					var price float64
					for _, ct := range clusterTypes {
						if ct.ClusterType == preferredType {
							price = float64(ct.PricePerHour)
							break
						}
					}
					t.Logf("✅ Preferred cluster type %s is AVAILABLE at %s ($%.2f/hr)", preferredType, loc, price)
					return &AvailableClusterType{
						ClusterType:  preferredType,
						PricePerHour: price,
						Location:     loc,
						Image:        defaultImage,
					}, true
				}
			}
		}
		t.Logf("⚠️  Preferred cluster type %s is NOT available", preferredType)
	}

	// Sort cluster types by price (cheapest first)
	sort.Slice(clusterTypes, func(i, j int) bool {
		return float64(clusterTypes[i].PricePerHour) < float64(clusterTypes[j].PricePerHour)
	})

	// Find the cheapest available cluster type
	for _, ct := range clusterTypes {
		if locs, ok := availableMap[ct.ClusterType]; ok {
			for loc, avail := range locs {
				if avail {
					t.Logf("✅ Found cheapest available: %s at %s ($%.2f/hr)", ct.ClusterType, loc, float64(ct.PricePerHour))
					return &AvailableClusterType{
						ClusterType:  ct.ClusterType,
						PricePerHour: float64(ct.PricePerHour),
						Location:     loc,
						Image:        defaultImage,
					}, true
				}
			}
		}
	}

	t.Log("❌ No cluster types available in staging environment")
	return nil, false
}

// FindAvailableContainerCompute checks availability for serverless containers
func FindAvailableContainerCompute(ctx context.Context, t *testing.T, client *verda.Client, preferredName string) (string, int, bool) {
	t.Helper()

	t.Log("🔍 Checking container compute availability...")

	resources, err := client.ContainerDeployments.GetServerlessComputeResources(ctx)
	if err != nil {
		t.Logf("⚠️  Could not get compute resources: %v", err)
		return "", 0, false
	}
	t.Logf("   Found %d compute resources", len(resources))

	// Check if preferred compute is available
	if preferredName != "" {
		for _, r := range resources {
			if r.Name == preferredName && r.IsAvailable {
				t.Logf("✅ Preferred compute %s (size %d) is AVAILABLE", r.Name, r.Size)
				return r.Name, 1, true
			}
		}
		t.Logf("⚠️  Preferred compute %s is NOT available", preferredName)
	}

	// Find any available compute
	for _, r := range resources {
		if r.IsAvailable {
			t.Logf("✅ Found available compute: %s (size %d)", r.Name, r.Size)
			return r.Name, 1, true
		}
	}

	t.Log("❌ No container compute resources available")
	return "", 0, false
}

// ============================================================================
// WAIT HELPERS
// ============================================================================

// WaitForInstanceStatus waits for an instance to reach a target status
func WaitForInstanceStatus(ctx context.Context, t *testing.T, client *verda.Client, instanceID string, targetStatus string, timeout time.Duration) (*verda.Instance, bool) {
	t.Helper()

	t.Logf("⏳ Waiting for instance %s to reach status '%s' (timeout: %v)...", instanceID, targetStatus, timeout)

	start := time.Now()
	lastStatus := ""
	for time.Since(start) < timeout {
		instance, err := client.Instances.GetByID(ctx, instanceID)
		if err != nil {
			t.Logf("   ⚠️  Error getting instance: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if instance.Status != lastStatus {
			t.Logf("   Instance status: %s", instance.Status)
			lastStatus = instance.Status
		}

		if instance.Status == targetStatus {
			t.Logf("✅ Instance reached target status '%s'", targetStatus)
			return instance, true
		}

		// If instance failed, stop waiting
		if instance.Status == "FAILED" || instance.Status == "failed" {
			t.Logf("❌ Instance failed")
			return instance, false
		}

		time.Sleep(10 * time.Second)
	}

	t.Logf("❌ Timeout waiting for instance to reach status '%s'", targetStatus)
	return nil, false
}

// WaitForClusterStatus waits for a cluster to reach a target status
func WaitForClusterStatus(ctx context.Context, t *testing.T, client *verda.Client, clusterID string, targetStatus string, timeout time.Duration) (*verda.Cluster, bool) {
	t.Helper()

	t.Logf("⏳ Waiting for cluster %s to reach status '%s' (timeout: %v)...", clusterID, targetStatus, timeout)

	start := time.Now()
	lastStatus := ""
	for time.Since(start) < timeout {
		cluster, err := client.Clusters.GetByID(ctx, clusterID)
		if err != nil {
			t.Logf("   ⚠️  Error getting cluster: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if cluster.Status != lastStatus {
			t.Logf("   Cluster status: %s", cluster.Status)
			lastStatus = cluster.Status
		}

		if cluster.Status == targetStatus {
			t.Logf("✅ Cluster reached target status '%s'", targetStatus)
			return cluster, true
		}

		// If cluster failed, stop waiting
		if cluster.Status == "FAILED" || cluster.Status == "failed" {
			t.Logf("❌ Cluster failed")
			return cluster, false
		}

		time.Sleep(10 * time.Second)
	}

	t.Logf("❌ Timeout waiting for cluster to reach status '%s'", targetStatus)
	return nil, false
}
