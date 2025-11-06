package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/verda-cloud/verda-go/pkg/verda"
)

func main() {
	// Get credentials from environment variables
	clientID := os.Getenv("VERDA_CLIENT_ID")
	clientSecret := os.Getenv("VERDA_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("VERDA_CLIENT_ID and VERDA_CLIENT_SECRET environment variables are required")
	}

	// Create client
	client, err := verda.NewClient(
		verda.WithClientID(clientID),
		verda.WithClientSecret(clientSecret))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

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
	keys, err := client.SSHKeys.Get(ctx)
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
			if location.Available {
				status = "available"
			}
			fmt.Printf("- %s (%s): %s - %s\n",
				location.Name, location.Code, location.Country, status)
		}
	}

	// Example: Check instance availability
	fmt.Println("\n=== Instance Availability ===")
	available, err := client.Instances.IsAvailable(ctx, "1V100.6V", false, "")
	if err != nil {
		log.Printf("Error checking availability: %v", err)
	} else {
		fmt.Printf("1V100.6V available: %v\n", available)
	}

	// Example: Create instance (commented out to avoid accidental creation)
	/*
		fmt.Println("\n=== Creating Instance ===")
		instance, err := client.Instances.Create(ctx, verda.CreateInstanceRequest{
			InstanceType: "1V100.6V",
			Image:        "ubuntu-24.04-cuda-12.8-open-docker",
			Hostname:     "test-instance",
			Description:  "Test instance from Go SDK",
			SSHKeyIDs:    []string{}, // Add your SSH key IDs here
			LocationCode: verda.LocationFIN01,
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
