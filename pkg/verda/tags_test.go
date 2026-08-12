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

package verda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestTagRequest_Validate(t *testing.T) {
	tests := []struct {
		name      string
		req       TagRequest
		expectErr bool
	}{
		{"key and value", TagRequest{Key: "environment", Value: "production"}, false},
		{"freeform tag without value", TagRequest{Key: "benchmark"}, false},
		{"missing key", TagRequest{Value: "production"}, true},
		{"key at max length", TagRequest{Key: strings.Repeat("a", TagKeyMaxLength)}, false},
		{"key over max length", TagRequest{Key: strings.Repeat("a", TagKeyMaxLength+1)}, true},
		{"value at max length", TagRequest{Key: "k", Value: strings.Repeat("a", TagValueMaxLength)}, false},
		{"value over max length", TagRequest{Key: "k", Value: strings.Repeat("a", TagValueMaxLength+1)}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestTagRequest_FreeformSerialization(t *testing.T) {
	t.Run("value omitted when empty", func(t *testing.T) {
		body, err := json.Marshal(TagRequest{Key: "benchmark"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(string(body), "value") {
			t.Errorf("expected value to be omitted for a freeform tag, got %s", body)
		}
	})

	t.Run("value included when set", func(t *testing.T) {
		body, err := json.Marshal(TagRequest{Key: "environment", Value: "production"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(body), `"value":"production"`) {
			t.Errorf("expected value in request body, got %s", body)
		}
	})
}

func TestInstanceService_AddTag(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)
	ctx := context.Background()

	t.Run("add tag with value", func(t *testing.T) {
		tag, err := client.Instances.AddTag(ctx, "instance_add_1", TagRequest{Key: "environment", Value: "production"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag.Key != "environment" {
			t.Errorf("expected key environment, got %s", tag.Key)
		}
		if tag.Value != "production" {
			t.Errorf("expected value production, got %s", tag.Value)
		}
		if tag.ID == "" {
			t.Error("expected tag to have an ID")
		}
	})

	t.Run("add freeform tag", func(t *testing.T) {
		tag, err := client.Instances.AddTag(ctx, "instance_add_2", TagRequest{Key: "benchmark"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag.Value != "" {
			t.Errorf("expected empty value for a freeform tag, got %q", tag.Value)
		}
	})

	t.Run("keys are lowercased by the API", func(t *testing.T) {
		tag, err := client.Instances.AddTag(ctx, "instance_add_3", TagRequest{Key: "Environment", Value: "Production"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag.Key != "environment" {
			t.Errorf("expected lowercased key, got %s", tag.Key)
		}
	})

	t.Run("duplicate key returns 409", func(t *testing.T) {
		id := "instance_dup"
		if _, err := client.Instances.AddTag(ctx, id, TagRequest{Key: "environment", Value: "staging"}); err != nil {
			t.Fatalf("unexpected error on first add: %v", err)
		}

		_, err := client.Instances.AddTag(ctx, id, TagRequest{Key: "environment", Value: "production"})
		if err == nil {
			t.Fatal("expected an error for a duplicate tag key, got nil")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusConflict {
			t.Errorf("expected status 409, got %d", apiErr.StatusCode)
		}
	})

	t.Run("invalid request is rejected before the request is sent", func(t *testing.T) {
		if _, err := client.Instances.AddTag(ctx, "instance_add_4", TagRequest{Value: "production"}); err == nil {
			t.Error("expected validation error for a missing key, got nil")
		}
	})

	t.Run("empty instance ID is rejected", func(t *testing.T) {
		if _, err := client.Instances.AddTag(ctx, "", TagRequest{Key: "environment"}); err == nil {
			t.Error("expected error for an empty instance ID, got nil")
		}
	})
}

func TestInstanceService_DeleteTag(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)
	ctx := context.Background()

	t.Run("delete existing tag", func(t *testing.T) {
		id := "instance_del_1"
		if _, err := client.Instances.AddTag(ctx, id, TagRequest{Key: "environment", Value: "production"}); err != nil {
			t.Fatalf("unexpected error adding tag: %v", err)
		}

		if err := client.Instances.DeleteTag(ctx, id, "environment"); err != nil {
			t.Fatalf("unexpected error deleting tag: %v", err)
		}

		if tags := mockServer.ResourceTags("/instances/" + id); len(tags) != 0 {
			t.Errorf("expected no tags left, got %d", len(tags))
		}
	})

	t.Run("unknown tag returns 404", func(t *testing.T) {
		err := client.Instances.DeleteTag(ctx, "instance_del_2", "nope")
		if err == nil {
			t.Fatal("expected an error for an unknown tag, got nil")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", apiErr.StatusCode)
		}
	})

	// Tag keys are arbitrary user-supplied text interpolated into the request
	// path. Unescaped, a key containing "?", "#" or "/" would silently target a
	// different URL than the caller asked for.
	for i, key := range []string{"team/platform", "cost?center", "note#1", "owner name"} {
		t.Run("key requiring escaping is sent intact: "+key, func(t *testing.T) {
			id := fmt.Sprintf("instance_escape_%d", i)

			if _, err := client.Instances.AddTag(ctx, id, TagRequest{Key: key, Value: "x"}); err != nil {
				t.Fatalf("unexpected error adding tag: %v", err)
			}

			if err := client.Instances.DeleteTag(ctx, id, key); err != nil {
				t.Fatalf("unexpected error deleting tag with key %q: %v", key, err)
			}

			if tags := mockServer.ResourceTags("/instances/" + id); len(tags) != 0 {
				t.Errorf("expected no tags left for key %q, got %d", key, len(tags))
			}
		})
	}

	t.Run("empty key is rejected", func(t *testing.T) {
		if err := client.Instances.DeleteTag(ctx, "instance_del_4", ""); err == nil {
			t.Error("expected error for an empty tag key, got nil")
		}
	})

	t.Run("empty instance ID is rejected", func(t *testing.T) {
		if err := client.Instances.DeleteTag(ctx, "", "environment"); err == nil {
			t.Error("expected error for an empty instance ID, got nil")
		}
	})
}

func TestVolumeService_Tags(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)
	ctx := context.Background()

	t.Run("add and delete a volume tag", func(t *testing.T) {
		id := "vol_123"

		tag, err := client.Volumes.AddTag(ctx, id, TagRequest{Key: "environment", Value: "production"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag.Key != "environment" {
			t.Errorf("expected key environment, got %s", tag.Key)
		}

		if tags := mockServer.ResourceTags("/volumes/" + id); len(tags) != 1 {
			t.Fatalf("expected 1 stored tag, got %d", len(tags))
		}

		if err := client.Volumes.DeleteTag(ctx, id, "environment"); err != nil {
			t.Fatalf("unexpected error deleting tag: %v", err)
		}
	})
}

func TestClusterService_Tags(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)
	ctx := context.Background()

	t.Run("add and delete a cluster tag", func(t *testing.T) {
		id := "cluster_123"

		tag, err := client.Clusters.AddTag(ctx, id, TagRequest{Key: "environment", Value: "production"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag.Key != "environment" {
			t.Errorf("expected key environment, got %s", tag.Key)
		}

		if err := client.Clusters.DeleteTag(ctx, id, "environment"); err != nil {
			t.Fatalf("unexpected error deleting tag: %v", err)
		}

		if tags := mockServer.ResourceTags("/clusters/" + id); len(tags) != 0 {
			t.Errorf("expected no tags left, got %d", len(tags))
		}
	})
}

func TestTagsInResourceResponses(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)
	ctx := context.Background()

	t.Run("instance response carries tags", func(t *testing.T) {
		id := "instance_resp_1"
		if _, err := client.Instances.AddTag(ctx, id, TagRequest{Key: "environment", Value: "production"}); err != nil {
			t.Fatalf("unexpected error adding tag: %v", err)
		}
		if _, err := client.Instances.AddTag(ctx, id, TagRequest{Key: "benchmark"}); err != nil {
			t.Fatalf("unexpected error adding freeform tag: %v", err)
		}

		instance, err := client.Instances.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(instance.Tags) != 2 {
			t.Fatalf("expected 2 tags on the instance, got %d", len(instance.Tags))
		}
		if instance.Tags[0].Key != "environment" || instance.Tags[0].Value != "production" {
			t.Errorf("unexpected first tag: %+v", instance.Tags[0])
		}
		if instance.Tags[1].Key != "benchmark" || instance.Tags[1].Value != "" {
			t.Errorf("expected a freeform tag with an empty value, got %+v", instance.Tags[1])
		}
	})

	t.Run("cluster response carries tags", func(t *testing.T) {
		id := "cluster_resp_1"
		if _, err := client.Clusters.AddTag(ctx, id, TagRequest{Key: "environment", Value: "staging"}); err != nil {
			t.Fatalf("unexpected error adding tag: %v", err)
		}

		cluster, err := client.Clusters.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cluster.Tags) != 1 {
			t.Fatalf("expected 1 tag on the cluster, got %d", len(cluster.Tags))
		}
		if cluster.Tags[0].Value != "staging" {
			t.Errorf("expected value staging, got %s", cluster.Tags[0].Value)
		}
	})

	t.Run("volume response decodes tags", func(t *testing.T) {
		mockServer.SetHandler(http.MethodGet, "/volumes/vol_tagged", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(w, map[string]interface{}{
				"id":   "vol_tagged",
				"name": "tagged-volume",
				"tags": []map[string]string{
					{"id": "tag_1", "key": "environment", "value": "production"},
				},
			})
		})

		volume, err := client.Volumes.GetVolume(ctx, "vol_tagged")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(volume.Tags) != 1 {
			t.Fatalf("expected 1 tag on the volume, got %d", len(volume.Tags))
		}
		if volume.Tags[0].Key != "environment" {
			t.Errorf("expected key environment, got %s", volume.Tags[0].Key)
		}
	})
}

func TestCreateRequestsCarryTags(t *testing.T) {
	t.Run("create instance request serializes tags", func(t *testing.T) {
		body, err := json.Marshal(CreateInstanceRequest{
			InstanceType: "1V100.6V",
			Image:        "ubuntu-24.04",
			Hostname:     "test",
			Description:  "test",
			Tags:         []TagRequest{{Key: "environment", Value: "production"}, {Key: "benchmark"}},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(body), `"tags":[{"key":"environment","value":"production"},{"key":"benchmark"}]`) {
			t.Errorf("unexpected tags serialization: %s", body)
		}
	})

	t.Run("tags omitted when unset", func(t *testing.T) {
		body, err := json.Marshal(VolumeCreateRequest{Name: "v", Size: 10, Type: VolumeTypeNVMe})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(string(body), "tags") {
			t.Errorf("expected tags to be omitted when unset, got %s", body)
		}
	})

	t.Run("more than the maximum number of tags is rejected", func(t *testing.T) {
		tags := make([]TagRequest, MaxTagsPerResource+1)
		for i := range tags {
			tags[i] = TagRequest{Key: string(rune('a' + i))}
		}

		req := CreateInstanceRequest{
			InstanceType: "1V100.6V",
			Image:        "ubuntu-24.04",
			Hostname:     "test",
			Description:  "test",
			Tags:         tags,
		}
		if err := req.Validate(); err == nil {
			t.Errorf("expected validation error for more than %d tags, got nil", MaxTagsPerResource)
		}
	})

	t.Run("invalid tag in a create request is rejected", func(t *testing.T) {
		req := CreateClusterRequest{
			ClusterType:  "8V100.48V",
			Image:        "ubuntu-22.04",
			Hostname:     "test",
			Description:  "test",
			SharedVolume: ClusterSharedVolumeSpec{Name: "shared", Size: 100},
			Tags:         []TagRequest{{Value: "no-key"}},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected validation error for a tag without a key, got nil")
		}
	})
}
