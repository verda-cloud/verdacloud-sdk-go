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
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

const instanceGroupsOptInEnv = "VERDA_E2E_INSTANCE_GROUPS"

func TestInstanceGroupsLifecycleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv(instanceGroupsOptInEnv) != "1" {
		t.Skipf("set %s=1 to run the experimental instance-group lifecycle", instanceGroupsOptInEnv)
	}

	client := getTestClient(t)
	ctx := t.Context()
	available, ok := FindAvailableInstanceType(ctx, t, client, PreferredInstanceType)
	if !ok {
		t.Skip("no instance types available at any location")
	}
	image, err := findInstanceImage(ctx, client, available.InstanceType)
	if err != nil {
		t.Fatalf("select instance image: %v", err)
	}

	var groupID string
	t.Cleanup(func() {
		if groupID == "" {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := client.InstanceGroups.Delete(cleanupCtx, groupID); err != nil {
			t.Errorf("delete instance group %s during cleanup: %v", groupID, err)
		}
	})

	description := "SDK experimental integration group"
	reserved, minSize, maxSize := 0, 0, 2
	minSizePointer, maxSizePointer := &minSize, &maxSize
	group, err := client.InstanceGroups.Create(ctx, verda.CreateInstanceGroupRequest{
		Name:         generateRandomName("sdk-instance-group"),
		Description:  &description,
		LocationCode: available.Location,
		InstanceType: available.InstanceType,
		Management:   verda.InstanceGroupManagementExternal,
		Interconnect: &verda.InstanceGroupInterconnectRequest{Type: verda.InstanceGroupInterconnectNone},
		Template:     verda.InstanceGroupTemplateRequest{Image: image},
		Capacity: &verda.InstanceGroupCapacityRequest{
			Reserved: &reserved,
			MinSize:  &minSizePointer,
			MaxSize:  &maxSizePointer,
		},
	})
	if err != nil {
		t.Fatalf("create instance group: %v", err)
	}
	groupID = group.ID
	if group.Size != 0 {
		t.Errorf("new group size = %d, want 0", group.Size)
	}

	groups, err := client.InstanceGroups.List(ctx)
	if err != nil {
		t.Fatalf("list instance groups: %v", err)
	}
	found := false
	for _, candidate := range groups {
		if candidate.ID == groupID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created group %s is absent from List", groupID)
	}

	group, err = client.InstanceGroups.GetByID(ctx, groupID)
	if err != nil {
		t.Fatalf("get instance group: %v", err)
	}
	if group.Template.Image != image || group.InstanceType != available.InstanceType || group.LocationCode != available.Location {
		t.Errorf("unexpected group response: %+v", group)
	}

	newName := group.Name + "-updated"
	var clearDescription *string
	maxSize = 4
	group, err = client.InstanceGroups.Update(ctx, groupID, verda.UpdateInstanceGroupRequest{
		Name:        &newName,
		Description: &clearDescription,
		Capacity:    &verda.InstanceGroupCapacityRequest{MaxSize: &maxSizePointer},
	})
	if err != nil {
		t.Fatalf("update instance group: %v", err)
	}
	if group.Name != newName || group.Description != nil || group.Capacity.MaxSize == nil || *group.Capacity.MaxSize != maxSize {
		t.Errorf("unexpected updated group: %+v", group)
	}

	if err := client.InstanceGroups.Delete(ctx, groupID); err != nil {
		t.Fatalf("delete instance group: %v", err)
	}
	_, err = client.InstanceGroups.GetByID(ctx, groupID)
	var apiErr *verda.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("get deleted group error = %v, want API 404", err)
	}
	groupID = ""
}
