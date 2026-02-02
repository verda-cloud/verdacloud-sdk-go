package verda

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestClusterService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get all clusters", func(t *testing.T) {
		ctx := context.Background()
		clusters, err := client.Clusters.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(clusters) == 0 {
			t.Error("expected at least one cluster")
		}
	})
}

func TestClusterService_GetByID(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get cluster by ID", func(t *testing.T) {
		ctx := context.Background()
		clusterID := "test_cluster_123"
		cluster, err := client.Clusters.GetByID(ctx, clusterID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if cluster == nil {
			t.Fatal("expected cluster, got nil")
		}

		if cluster.ID != clusterID {
			t.Errorf("expected cluster ID '%s', got '%s'", clusterID, cluster.ID)
		}
	})
}

func TestClusterService_Create(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("create cluster with minimal config", func(t *testing.T) {
		req := CreateClusterRequest{
			ClusterType: "8V100.48V",
			Image:       "ubuntu-22.04-cuda-12.0",
			Hostname:    "test-cluster",
			Description: "Test cluster",
			SSHKeyIDs:   []string{"key_123"},
			SharedVolume: ClusterSharedVolumeSpec{
				Name: "test-volume",
				Size: 1000,
			},
		}

		ctx := context.Background()
		response, err := client.Clusters.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if response == nil {
			t.Fatal("expected response, got nil")
		}

		if response.ID == "" {
			t.Error("expected cluster ID, got empty string")
		}
	})

	t.Run("create cluster with full config", func(t *testing.T) {
		startupScriptID := "script_123"
		autoRentExtension := true
		turnToPayAsYouGo := false

		req := CreateClusterRequest{
			ClusterType:       "8V100.48V",
			Image:             "ubuntu-22.04-cuda-12.0",
			Hostname:          "test-cluster-full",
			Description:       "Test cluster with full config",
			SSHKeyIDs:         []string{"key_123", "key_456"},
			LocationCode:      LocationFIN01,
			Contract:          "PAY_AS_YOU_GO",
			StartupScriptID:   &startupScriptID,
			AutoRentExtension: &autoRentExtension,
			TurnToPayAsYouGo:  &turnToPayAsYouGo,
			SharedVolume: ClusterSharedVolumeSpec{
				Name: "cluster-volume",
				Size: 5000,
			},
			ExistingVolumes: []ClusterExistingVolume{
				{ID: "vol_456"},
			},
		}

		ctx := context.Background()
		response, err := client.Clusters.Create(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if response == nil {
			t.Fatal("expected response, got nil")
		}
	})
}

func TestClusterService_Discontinue(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("discontinue single cluster", func(t *testing.T) {
		ctx := context.Background()
		err := client.Clusters.Discontinue(ctx, []string{"cluster_123"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("discontinue multiple clusters", func(t *testing.T) {
		ctx := context.Background()
		clusterIDs := []string{"cluster_123", "cluster_456"}
		err := client.Clusters.Discontinue(ctx, clusterIDs)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestClusterService_GetClusterTypes(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get cluster types with default currency", func(t *testing.T) {
		ctx := context.Background()
		clusterTypes, err := client.Clusters.GetClusterTypes(ctx, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(clusterTypes) == 0 {
			t.Error("expected at least one cluster type")
		}

		// Verify structure matches API documentation
		for _, ct := range clusterTypes {
			if ct.ID == "" {
				t.Error("expected non-empty ID")
			}
			if ct.Model == "" {
				t.Error("expected non-empty Model")
			}
			if ct.Name == "" {
				t.Error("expected non-empty Name")
			}
			if ct.ClusterType == "" {
				t.Error("expected non-empty ClusterType")
			}
			if ct.Currency == "" {
				t.Error("expected non-empty Currency")
			}
			if ct.Manufacturer == "" {
				t.Error("expected non-empty Manufacturer")
			}
			if len(ct.NodeDetails) == 0 {
				t.Error("expected at least one node detail")
			}
			if len(ct.SupportedOS) == 0 {
				t.Error("expected at least one supported OS")
			}
		}
	})

	t.Run("get cluster types with USD currency", func(t *testing.T) {
		ctx := context.Background()
		clusterTypes, err := client.Clusters.GetClusterTypes(ctx, "usd")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(clusterTypes) == 0 {
			t.Error("expected at least one cluster type")
		}
	})
}

func TestClusterService_GetAvailabilities(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get all cluster availabilities", func(t *testing.T) {
		ctx := context.Background()
		availabilities, err := client.Clusters.GetAvailabilities(ctx, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(availabilities) == 0 {
			t.Error("expected at least one availability entry")
		}

		// Verify structure matches API documentation
		for _, avail := range availabilities {
			if avail.LocationCode == "" {
				t.Error("expected non-empty LocationCode")
			}
			if len(avail.Availabilities) == 0 {
				t.Error("expected at least one available cluster type")
			}
		}
	})

	t.Run("get cluster availabilities for specific location", func(t *testing.T) {
		ctx := context.Background()
		availabilities, err := client.Clusters.GetAvailabilities(ctx, LocationFIN01)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(availabilities) == 0 {
			t.Error("expected at least one availability entry")
		}
	})
}

func TestClusterService_CheckClusterTypeAvailability(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("check cluster type availability", func(t *testing.T) {
		ctx := context.Background()
		available, err := client.Clusters.CheckClusterTypeAvailability(ctx, "8V100.48V", "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// We just check that the call succeeded
		_ = available
	})

	t.Run("check cluster type availability for specific location", func(t *testing.T) {
		ctx := context.Background()
		available, err := client.Clusters.CheckClusterTypeAvailability(ctx, "8V100.48V", LocationFIN01)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// We just check that the call succeeded
		_ = available
	})
}

func TestClusterService_GetImages(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get cluster images", func(t *testing.T) {
		ctx := context.Background()
		images, err := client.Clusters.GetImages(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(images) == 0 {
			t.Error("expected at least one cluster image")
		}

		// Verify structure matches API documentation
		for _, img := range images {
			if img.ID == "" {
				t.Error("expected non-empty ID")
			}
			if img.ImageType == "" {
				t.Error("expected non-empty ImageType")
			}
			if img.Name == "" {
				t.Error("expected non-empty Name")
			}
			if img.Category == "" {
				t.Error("expected non-empty Category")
			}
			if len(img.Details) == 0 {
				t.Error("expected at least one detail")
			}
			// IsDefault and IsCluster are booleans, no validation needed
		}
	})
}
