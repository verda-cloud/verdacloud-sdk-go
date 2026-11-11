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

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

// findInstanceImage returns the image_type value that instance creation requires.
func findInstanceImage(ctx context.Context, client *verda.Client, instanceType string) (string, error) {
	images, err := client.Images.GetImagesByInstanceType(ctx, instanceType)
	if err != nil {
		return "", fmt.Errorf("get images for instance type %q: %w", instanceType, err)
	}

	var fallback string
	for _, image := range images {
		if image.ImageType == "" || image.IsCluster {
			continue
		}
		if image.IsDefault {
			return image.ImageType, nil
		}
		if fallback == "" {
			fallback = image.ImageType
		}
	}
	if fallback == "" {
		return "", fmt.Errorf("no usable images for instance type %q", instanceType)
	}
	return fallback, nil
}

func TestFindInstanceImage(t *testing.T) {
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
				{ImageType: "fallback"},
				{ID: "image-id", ImageType: "current-default", Name: "Jupyter", IsDefault: true},
				{ImageType: "another-default", IsDefault: true},
			},
			want: "current-default",
		},
		{
			name: "fall back in API order",
			images: []verda.Image{
				{ImageType: "z-first"},
				{ImageType: "a-second"},
			},
			want: "z-first",
		},
		{
			name: "ignore missing image types and cluster defaults",
			images: []verda.Image{
				{ID: "missing-type-id", Name: "Display name only", IsDefault: true},
				{ImageType: "cluster-default", IsDefault: true, IsCluster: true},
				{ImageType: "instance-default", IsDefault: true},
			},
			want: "instance-default",
		},
		{
			name: "ignore unusable fallback images",
			images: []verda.Image{
				{},
				{ImageType: "cluster", IsCluster: true},
				{ImageType: "instance"},
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
				{ID: "missing-type-id", Name: "Display name only", IsDefault: true},
				{ImageType: "cluster", IsCluster: true},
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
