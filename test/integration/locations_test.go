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
)

func TestLocations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := getTestClient(t)

	t.Run("get_locations", func(t *testing.T) {
		ctx := context.Background()
		locations, err := client.Locations.Get(ctx)
		if err != nil {
			t.Errorf("failed to get locations: %v", err)
		}
		if len(locations) == 0 {
			t.Error("expected at least one location")
		}
		t.Logf("Found %d locations", len(locations))
	})
}
