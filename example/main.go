package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func main() {
	// Get credentials from environment variables
	clientID := os.Getenv("VERDA_CLIENT_ID")
	clientSecret := os.Getenv("VERDA_CLIENT_SECRET")
	baseURL := os.Getenv("VERDA_BASE_URL")

	if clientID == "" || clientSecret == "" {
		log.Fatal("VERDA_CLIENT_ID and VERDA_CLIENT_SECRET environment variables are required")
	}

	// Create client
	client, err := verda.NewClient(
		verda.WithDebugLogging(true),
		verda.WithBaseURL(baseURL),
		verda.WithClientID(clientID),
		verda.WithClientSecret(clientSecret))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Add detailed debug logging to see full request/response payloads
	verda.AddDetailedDebugLogging(client)

	// Example: Get account balance
	fmt.Println("=== Account Balance ===")
	ctx := context.Background()
	balance, err := client.Balance.Get(ctx)
	if err != nil {
		log.Printf("Error getting balance: %v", err)
	} else {
		fmt.Printf("Balance: %.2f %s\n", balance.Amount, balance.Currency)
	}

	// Example: List all instances
	fmt.Println("\n=== Instances ===")
	instances, err := client.Instances.Get(ctx, "")
	if err != nil {
		log.Printf("Error getting instances: %v", err)
	} else {
		fmt.Printf("Found %d instances:\n", len(instances))
		for _, instance := range instances {
			fmt.Printf("- %s (%s): %s - %s\n",
				instance.Hostname, instance.ID, instance.InstanceType, instance.Status)
		}
	}

	// Example: List SSH keys
	fmt.Println("\n=== SSH Keys ===")
	keys, err := client.SSHKeys.GetAllSSHKeys(ctx)
	if err != nil {
		log.Printf("Error getting SSH keys: %v", err)
	} else {
		fmt.Printf("Found %d SSH keys:\n", len(keys))
		for _, key := range keys {
			fmt.Printf("- %s (%s)\n", key.Name, key.ID)
		}
	}

	// Example: List locations
	fmt.Println("\n=== Locations ===")
	locations, err := client.Locations.Get(ctx)
	if err != nil {
		log.Printf("Error getting locations: %v", err)
	} else {
		fmt.Printf("Available locations:\n")
		for _, location := range locations {
			status := "unavailable"
			if location.Code == verda.LocationFIN03 {
				status = "available"
			}
			fmt.Printf("- %s (%s): %s - %s\n",
				location.Name, location.Code, location.CountryCode, status)
		}
	}

	// Example: Check instance availability
	fmt.Println("\n=== Instance Availability ===")
	available, err := client.Instances.CheckInstanceTypeAvailability(ctx, "1V100.6V")
	if err != nil {
		log.Printf("Error checking availability: %v", err)
	} else {
		fmt.Printf("1V100.6V available: %v\n", available)
	}

	// Example: List cluster types
	fmt.Println("\n=== Cluster Types ===")
	clusterTypes, err := client.Clusters.GetClusterTypes(ctx, "usd")
	if err != nil {
		log.Printf("Error getting cluster types: %v", err)
	} else {
		fmt.Printf("Found %d cluster types:\n", len(clusterTypes))
		for i, ct := range clusterTypes {
			if i < 5 { // Show first 5
				fmt.Printf("- %s (%s): $%.2f/hr - %s\n",
					ct.ClusterType, ct.Model, ct.PricePerHour.Float64(), ct.Name)
			}
		}
	}

	// Example: Check cluster availability
	fmt.Println("\n=== Cluster Availability ===")
	clusterAvailabilities, err := client.Clusters.GetAvailabilities(ctx, verda.LocationFIN03)
	if err != nil {
		log.Printf("Error getting cluster availability: %v", err)
	} else {
		fmt.Printf("Cluster availability at %s:\n", verda.LocationFIN03)
		for _, avail := range clusterAvailabilities {
			fmt.Printf("- Location %s: %d cluster types available\n",
				avail.LocationCode, len(avail.Availabilities))
			for _, clusterType := range avail.Availabilities {
				fmt.Printf("  - %s\n", clusterType)
			}
		}
	}

	// Example: List cluster images
	fmt.Println("\n=== Cluster Images ===")
	clusterImages, err := client.Clusters.GetImages(ctx)
	if err != nil {
		log.Printf("Error getting cluster images: %v", err)
	} else {
		fmt.Printf("Available cluster images:\n")
		for _, img := range clusterImages {
			fmt.Printf("- %s (%s): %s\n", img.Name, img.ImageType, img.Category)
		}
	}

	// Example: List existing clusters
	fmt.Println("\n=== Clusters ===")
	clusters, err := client.Clusters.Get(ctx)
	if err != nil {
		log.Printf("Error getting clusters: %v", err)
	} else {
		fmt.Printf("Found %d clusters:\n", len(clusters))
		for _, cluster := range clusters {
			fmt.Printf("- %s (%s): %s - %s\n",
				cluster.Hostname, cluster.ID, cluster.ClusterType, cluster.Status)
		}
	}

	// Example: Create cluster (commented out to avoid accidental creation)
	/*
		fmt.Println("\n=== Creating Cluster ===")
		// Make sure you have SSH keys first
		if len(keys) == 0 {
			log.Println("⚠️  No SSH keys found. Create SSH keys before creating clusters.")
		} else {
			sshKeyIDs := []string{keys[0].ID}
			clusterReq := verda.CreateClusterRequest{
				ClusterType:  "16H200",
				Image:        "ubuntu-24.04-cuda-13.0-open",
				Hostname:     "my-test-cluster",
				Description:  "Test cluster from Go SDK",
				SSHKeyIDs:    sshKeyIDs,
				LocationCode: verda.LocationFIN03,
				Contract:     "hourly",
				Pricing:      "on-demand",
			}

			clusterResp, err := client.Clusters.Create(ctx, clusterReq)
			if err != nil {
				log.Printf("Error creating cluster: %v", err)
			} else {
				fmt.Printf("✅ Created cluster with ID: %s\n", clusterResp.ID)

				// Wait and check status
				cluster, err := client.Clusters.GetByID(ctx, clusterResp.ID)
				if err != nil {
					log.Printf("Error getting cluster: %v", err)
				} else {
					fmt.Printf("Cluster status: %s\n", cluster.Status)
				}

				// Cleanup (uncomment to delete after testing)
				// fmt.Println("\n=== Cleaning up cluster ===")
				// err = client.Clusters.Discontinue(ctx, []string{clusterResp.ID})
				// if err != nil {
				//     log.Printf("Error discontinuing cluster: %v", err)
				// } else {
				//     fmt.Printf("✅ Discontinued cluster: %s\n", clusterResp.ID)
				// }
			}
		}
	*/

	// Example: Create instance (commented out to avoid accidental creation)
	/*
		fmt.Println("\n=== Creating Instance ===")
		instance, err := client.Instances.Create(ctx, verda.CreateInstanceRequest{
			InstanceType: "1V100.6V",
			Image:        "ubuntu-24.04-cuda-12.8-open-docker",
			Hostname:     "test-instance",
			Description:  "Test instance from Go SDK",
			SSHKeyIDs:    []string{}, // Add your SSH key IDs here
			LocationCode: verda.LocationFIN03,
			Contract:     "PAY_AS_YOU_GO",
			Pricing:      "FIXED_PRICE",
		})
		if err != nil {
			log.Printf("Error creating instance: %v", err)
		} else {
			fmt.Printf("Created instance: %s (%s)\n", instance.Hostname, instance.ID)
		}
	*/
}
