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

package integration

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

// Keep the instance CRUD prefix so the smoke-test selector also runs these checks.
func TestInstanceCRUDIntegrationImageSelection(t *testing.T) {
	const instanceType = "CPU.4V.16G"
	tests := []struct {
		name    string
		images  []verda.Image
		status  int
		want    string
		wantErr string
	}{
		{
			name: "prefer first default over earlier non-default",
			images: []verda.Image{
				{Name: "fallback"},
				{Name: "current-default", IsDefault: true},
				{Name: "another-default", IsDefault: true},
			},
			want: "current-default",
		},
		{
			name: "fall back in API order",
			images: []verda.Image{
				{Name: "z-first"},
				{Name: "a-second"},
			},
			want: "z-first",
		},
		{
			name: "ignore unnamed and cluster defaults",
			images: []verda.Image{
				{IsDefault: true},
				{Name: "cluster-default", IsDefault: true, IsCluster: true},
				{Name: "instance-default", IsDefault: true},
			},
			want: "instance-default",
		},
		{
			name: "ignore unusable fallback images",
			images: []verda.Image{
				{},
				{Name: "cluster", IsCluster: true},
				{Name: "instance"},
			},
			want: "instance",
		},
		{
			name:    "empty list",
			images:  []verda.Image{},
			wantErr: "no usable images",
		},
		{
			name:    "null list",
			wantErr: "no usable images",
		},
		{
			name: "no usable images",
			images: []verda.Image{
				{IsDefault: true},
				{Name: "cluster", IsCluster: true},
			},
			wantErr: "no usable images",
		},
		{
			name:    "API error",
			status:  http.StatusServiceUnavailable,
			wantErr: "get images",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := testutil.NewMockServer()
			t.Cleanup(server.Close)
			client := verda.NewTestClient(t, server)
			server.SetHandler(http.MethodGet, "/images", func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("instance_type"); got != instanceType {
					t.Errorf("instance_type = %q, want %q", got, instanceType)
				}
				if tt.status != 0 {
					testutil.ErrorResponse(w, tt.status, "image catalog unavailable")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(tt.images); err != nil {
					t.Errorf("encode images: %v", err)
				}
			})

			got, err := findInstanceImage(t.Context(), client, instanceType)
			if got != tt.want {
				t.Errorf("image = %q, want %q", got, tt.want)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), instanceType) {
				t.Fatalf("error = %v, want %q and instance type %q", err, tt.wantErr, instanceType)
			}
			if tt.status != 0 {
				apiErr, ok := errors.AsType[*verda.APIError](err)
				if !ok || apiErr.StatusCode != tt.status {
					t.Errorf("error = %v, want APIError with status %d", err, tt.status)
				}
			}
		})
	}
}
