// Copyright 2026 Verda Cloud Oy
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestContainerTypesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := getTestClient(t)
	ctx := context.Background()

	t.Run("get container types", func(t *testing.T) {
		containerTypes, err := client.ContainerTypes.Get(ctx, "usd")
		if err != nil {
			// Check if it's a 404 error (not supported on staging)
			if apiErr, ok := err.(*verda.APIError); ok && apiErr.StatusCode == 404 {
				t.Skip("Container types endpoint not available (404)")
				return
			}
			t.Errorf("failed to get container types: %v", err)
		}
		t.Logf("Found %d container types", len(containerTypes))
	})
}
