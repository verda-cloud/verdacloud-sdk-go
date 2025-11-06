//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verda-go/pkg/verda"
)

func TestStartupScripts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := createTestClient(t)

	t.Run("get_startup_scripts", func(t *testing.T) {
		ctx := context.Background()
		scripts, err := client.StartupScripts.Get(ctx)
		if err != nil {
			// Check if it's a 404 error (not supported on staging)
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Logf("Startup scripts endpoint not available (404) - skipping test")
				return
			}
			t.Errorf("failed to get startup scripts: %v", err)
		}
		t.Logf("Found %d startup scripts", len(scripts))
	})
}

// TestCreateStartScript_Integration tests creating a startup script
func TestCreateStartScript_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createTestClient(t)

	ctx := context.Background()
	scriptID, err := client.StartupScripts.Create(ctx, verda.CreateStartupScriptRequest{
		Name:   "My startup script",
		Script: "#!/bin/bash\n\necho hello world",
	})

	if err != nil {
		// Check if it's a 404 error (not supported on staging)
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
			t.Logf("Startup scripts endpoint not available (404) - skipping test")
			return
		}
		t.Fatalf("failed to create start script: %v", err)
	}

	if scriptID.ID == "" {
		t.Fatal("created startup script has empty ID")
	}

	t.Logf("Created start script with ID: %s", scriptID.ID)

	// Cleanup: Delete the start script
	defer func() {
		t.Log("Cleaning up test start script...")
		err := client.StartupScripts.Delete(ctx, scriptID.ID)
		if err != nil {
			t.Errorf("failed to delete test start script %s: %v", scriptID.ID, err)
		} else {
			t.Log("Successfully cleaned up test start script")
		}
	}()

	// Verify the script can be retrieved
	retrievedScript, err := client.StartupScripts.GetByID(ctx, scriptID.ID)
	if err != nil {
		t.Errorf("failed to get created startup script: %v", err)
	} else {
		if retrievedScript.Name != "My startup script" {
			t.Errorf("expected script name 'My startup script', got %s", retrievedScript.Name)
		}
		if retrievedScript.Script != "#!/bin/bash\n\necho hello world" {
			t.Errorf("expected script content '#!/bin/bash\\n\\necho hello world', got %s", retrievedScript.Script)
		}
		t.Logf("Successfully retrieved startup script: %+v", retrievedScript)
	}
}

// TestListStartScripts_Integration tests listing startup scripts and finding a created one
func TestListStartScripts_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createTestClient(t)

	// Create a start script
	ctx := context.Background()
	scriptID, err := client.StartupScripts.Create(ctx, verda.CreateStartupScriptRequest{
		Name:   "My startup script for listing",
		Script: "#!/bin/bash\n\necho hello world from list test",
	})
	if err != nil {
		// Check if it's a 404 error (not supported on staging)
		if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
			t.Logf("Startup scripts endpoint not available (404) - skipping test")
			return
		}
		t.Fatalf("failed to create start script: %v", err)
	}
	t.Logf("Created start script with ID: %s", scriptID.ID)

	startScripts, err := client.StartupScripts.Get(ctx)
	if err != nil {
		t.Fatalf("failed to list start scripts: %v", err)
	}

	var found bool
	// Look for scriptID in the list
	for _, script := range startScripts {
		if script.ID == scriptID.ID {
			found = true
			if script.Name != "My startup script for listing" {
				t.Errorf("expected script name 'My startup script for listing', got %s", script.Name)
			}
			t.Logf("Found start script with ID: %s, Name: %s", scriptID.ID, script.Name)
			break
		}
	}

	if !found {
		t.Errorf("start script with ID %s not found in list of %d scripts", scriptID.ID, len(startScripts))
	}

	t.Logf("Successfully found start script with ID: %s in list of %d scripts", scriptID.ID, len(startScripts))

	// Cleanup
	defer func() {
		t.Log("Cleaning up test start script...")
		err := client.StartupScripts.Delete(ctx, scriptID.ID)
		if err != nil {
			t.Errorf("failed to delete test start script %s: %v", scriptID.ID, err)
		} else {
			t.Log("Successfully cleaned up test start script")
		}
	}()
}

// TestStartupScriptLifecycle_Integration tests the full lifecycle of startup scripts
func TestStartupScriptLifecycle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createTestClient(t)
	ctx := context.Background()

	// Test creating multiple scripts
	scripts := []struct {
		name   string
		script string
	}{
		{
			name:   "Test Script 1",
			script: "#!/bin/bash\necho 'Script 1 executed'",
		},
		{
			name:   "Test Script 2",
			script: "#!/bin/sh\ndate > /tmp/script2.log\necho 'Script 2 completed'",
		},
	}

	var createdScriptIDs []string

	// Create scripts
	for _, s := range scripts {
		scriptID, err := client.StartupScripts.Create(ctx, verda.CreateStartupScriptRequest{
			Name:   s.name,
			Script: s.script,
		})
		if err != nil {
			// Check if it's a 404 error (not supported on staging)
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Logf("Startup scripts endpoint not available (404) - skipping test")
				return
			}
			t.Fatalf("failed to create startup script %s: %v", s.name, err)
		}

		createdScriptIDs = append(createdScriptIDs, scriptID.ID)
		t.Logf("Created startup script '%s' with ID: %s", s.name, scriptID.ID)
	}

	// Verify all scripts exist in the list
	allScripts, err := client.StartupScripts.Get(ctx)
	if err != nil {
		t.Fatalf("failed to list startup scripts: %v", err)
	}

	for i, scriptID := range createdScriptIDs {
		found := false
		for _, script := range allScripts {
			if script.ID == scriptID {
				found = true
				if script.Name != scripts[i].name {
					t.Errorf("expected script name '%s', got '%s'", scripts[i].name, script.Name)
				}
				break
			}
		}
		if !found {
			t.Errorf("created script %s not found in list", scriptID)
		}
	}

	t.Logf("All %d created scripts found in list", len(createdScriptIDs))

	// Cleanup all created scripts
	defer func() {
		for i, scriptID := range createdScriptIDs {
			t.Logf("Cleaning up startup script %s (%s)...", scripts[i].name, scriptID)
			err := client.StartupScripts.Delete(ctx, scriptID)
			if err != nil {
				t.Errorf("failed to delete startup script %s: %v", scriptID, err)
			} else {
				t.Logf("Successfully cleaned up startup script %s", scriptID)
			}
		}
	}()
}
