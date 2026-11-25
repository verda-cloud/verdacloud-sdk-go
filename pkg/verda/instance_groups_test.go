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
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

const testInstanceGroupResponseJSON = `{
	"id": "group_123",
	"name": "hive-workers-a",
	"description": null,
	"project_id": "project_123",
	"location_code": "FIN-02",
	"instance_type": "8V100.80S",
	"management": "EXTERNAL",
	"size": 1,
	"interconnect": {"type": "INFINIBAND", "domain": "f3-isl-bb02"},
	"template": {
		"image": "ubuntu-24.04",
		"os_volume": {"size": 200},
		"ssh_key_ids": ["key_a"],
		"startup_script_id": null
	},
	"capacity": {
		"reserved": 2,
		"reserved_free": 1,
		"min_size": 0,
		"max_size": 16,
		"enforce_max_size": false
	},
	"reservations": [
		{"state": "occupied", "instance_id": "instance_123"},
		{"state": "spot_backfill", "instance_id": null}
	],
	"instances": [
		{"id": "instance_123", "hostname": "worker-1", "status": "running", "on_reserved": true}
	],
	"created_at": "2026-08-12T10:00:00Z",
	"updated_at": "2026-08-12T11:00:00Z",
	"deleted_at": null
}`

func writeTestInstanceGroupResponse(t *testing.T, w http.ResponseWriter, status int, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("write mock response: %v", err)
	}
}

func checkTestInstanceGroup(t *testing.T, group *InstanceGroup) {
	t.Helper()
	if group.ID != "group_123" || group.Name != "hive-workers-a" || group.ProjectID != "project_123" {
		t.Errorf("unexpected identity fields: %+v", group)
	}
	if group.Description != nil || group.DeletedAt != nil {
		t.Errorf("expected null description and deleted_at: %+v", group)
	}
	if group.LocationCode != "FIN-02" || group.InstanceType != "8V100.80S" || group.Management != InstanceGroupManagementExternal || group.Size != 1 {
		t.Errorf("unexpected group configuration: %+v", group)
	}
	if group.Interconnect.Type != InstanceGroupInterconnectInfiniBand || group.Interconnect.Domain == nil || *group.Interconnect.Domain != "f3-isl-bb02" {
		t.Errorf("unexpected interconnect: %+v", group.Interconnect)
	}
	if group.Template.Image != "ubuntu-24.04" || group.Template.OSVolume == nil || group.Template.OSVolume.Size != 200 {
		t.Errorf("unexpected template: %+v", group.Template)
	}
	if !reflect.DeepEqual(group.Template.SSHKeyIDs, []string{"key_a"}) || group.Template.StartupScriptID != nil {
		t.Errorf("unexpected template defaults: %+v", group.Template)
	}
	if group.Capacity.Reserved != 2 || group.Capacity.ReservedFree != 1 || group.Capacity.EnforceMaxSize {
		t.Errorf("unexpected capacity: %+v", group.Capacity)
	}
	if group.Capacity.MinSize == nil || *group.Capacity.MinSize != 0 || group.Capacity.MaxSize == nil || *group.Capacity.MaxSize != 16 {
		t.Errorf("unexpected capacity bounds: %+v", group.Capacity)
	}
	if len(group.Reservations) != 2 || group.Reservations[0].State != InstanceGroupReservationStateOccupied || group.Reservations[1].State != InstanceGroupReservationStateSpotBackfill {
		t.Fatalf("unexpected reservations: %+v", group.Reservations)
	}
	if group.Reservations[0].InstanceID == nil || *group.Reservations[0].InstanceID != "instance_123" || group.Reservations[1].InstanceID != nil {
		t.Errorf("unexpected reservation instance IDs: %+v", group.Reservations)
	}
	if len(group.Instances) != 1 || group.Instances[0].ID != "instance_123" || group.Instances[0].Hostname != "worker-1" || group.Instances[0].Status != StatusRunning || !group.Instances[0].OnReserved {
		t.Errorf("unexpected instances: %+v", group.Instances)
	}
	if !group.CreatedAt.Equal(time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)) || !group.UpdatedAt.Equal(time.Date(2026, 8, 12, 11, 0, 0, 0, time.UTC)) {
		t.Errorf("unexpected timestamps: %s, %s", group.CreatedAt, group.UpdatedAt)
	}
}

func TestInstanceGroupCapacityNullableBounds(t *testing.T) {
	var zeroBounds InstanceGroupCapacity
	if err := json.Unmarshal([]byte(`{"min_size":0,"max_size":0}`), &zeroBounds); err != nil {
		t.Fatalf("decode zero bounds: %v", err)
	}
	if zeroBounds.MinSize == nil || *zeroBounds.MinSize != 0 || zeroBounds.MaxSize == nil || *zeroBounds.MaxSize != 0 {
		t.Errorf("explicit zero bounds were not preserved: %+v", zeroBounds)
	}

	var nullBounds InstanceGroupCapacity
	if err := json.Unmarshal([]byte(`{"min_size":null,"max_size":null}`), &nullBounds); err != nil {
		t.Fatalf("decode null bounds: %v", err)
	}
	if nullBounds.MinSize != nil || nullBounds.MaxSize != nil {
		t.Errorf("null bounds decoded as non-nil: %+v", nullBounds)
	}
}

func TestInstanceGroupServiceCRUD(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)
	ctx := context.Background()

	mockServer.SetHandler(http.MethodGet, instanceGroupsBasePath, func(w http.ResponseWriter, r *http.Request) {
		writeTestInstanceGroupResponse(t, w, http.StatusOK, "["+testInstanceGroupResponseJSON+"]")
	})
	mockServer.SetHandler(http.MethodGet, instanceGroupsBasePath+"/group_123", func(w http.ResponseWriter, r *http.Request) {
		writeTestInstanceGroupResponse(t, w, http.StatusOK, testInstanceGroupResponseJSON)
	})
	mockServer.SetHandler(http.MethodPost, instanceGroupsBasePath, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode create request: %v", err)
		}
		want := map[string]any{
			"name":          "hive-workers-a",
			"description":   "worker group",
			"location_code": "FIN-02",
			"instance_type": "8V100.80S",
			"management":    "EXTERNAL",
			"interconnect":  map[string]any{"type": "NONE"},
			"template":      map[string]any{"image": "ubuntu-24.04", "os_volume": map[string]any{"size": float64(200)}},
			"capacity":      map[string]any{"reserved": float64(0), "min_size": float64(0), "max_size": float64(5), "enforce_max_size": false},
		}
		if !reflect.DeepEqual(body, want) {
			t.Errorf("create body = %#v, want %#v", body, want)
		}
		writeTestInstanceGroupResponse(t, w, http.StatusCreated, testInstanceGroupResponseJSON)
	})
	mockServer.SetHandler(http.MethodPatch, instanceGroupsBasePath+"/group_123", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode update request: %v", err)
		}
		want := map[string]any{
			"name":        "hive-workers-b",
			"description": nil,
			"template": map[string]any{
				"ssh_key_ids":       []any{},
				"startup_script_id": nil,
			},
			"capacity": map[string]any{
				"reserved":         float64(0),
				"min_size":         nil,
				"max_size":         float64(10),
				"enforce_max_size": false,
			},
		}
		if !reflect.DeepEqual(body, want) {
			t.Errorf("update body = %#v, want %#v", body, want)
		}
		writeTestInstanceGroupResponse(t, w, http.StatusOK, testInstanceGroupResponseJSON)
	})
	mockServer.SetHandler(http.MethodDelete, instanceGroupsBasePath+"/group_123", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	groups, err := client.InstanceGroups.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("List returned %d groups, want 1", len(groups))
	}
	checkTestInstanceGroup(t, &groups[0])

	group, err := client.InstanceGroups.GetByID(ctx, "group_123")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	checkTestInstanceGroup(t, group)

	description := "worker group"
	reserved, minSize, maxSize := 0, 0, 5
	minSizePointer, maxSizePointer := &minSize, &maxSize
	enforceMaxSize := false
	group, err = client.InstanceGroups.Create(ctx, CreateInstanceGroupRequest{
		Name:         "hive-workers-a",
		Description:  &description,
		LocationCode: "FIN-02",
		InstanceType: "8V100.80S",
		Management:   InstanceGroupManagementExternal,
		Interconnect: &InstanceGroupInterconnectRequest{Type: InstanceGroupInterconnectNone},
		Template: InstanceGroupTemplateRequest{
			Image:    "ubuntu-24.04",
			OSVolume: &InstanceGroupOSVolume{Size: 200},
		},
		Capacity: &InstanceGroupCapacityRequest{
			Reserved:       &reserved,
			MinSize:        &minSizePointer,
			MaxSize:        &maxSizePointer,
			EnforceMaxSize: &enforceMaxSize,
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	checkTestInstanceGroup(t, group)

	newName := "hive-workers-b"
	var clearDescription, clearStartupScript *string
	emptySSHKeys := []string{}
	maxSize = 10
	group, err = client.InstanceGroups.Update(ctx, "group_123", UpdateInstanceGroupRequest{
		Name:        &newName,
		Description: &clearDescription,
		Template: &UpdateInstanceGroupTemplateRequest{
			SSHKeyIDs:       &emptySSHKeys,
			StartupScriptID: &clearStartupScript,
		},
		Capacity: &InstanceGroupCapacityRequest{
			Reserved:       &reserved,
			MinSize:        new(*int),
			MaxSize:        &maxSizePointer,
			EnforceMaxSize: &enforceMaxSize,
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	checkTestInstanceGroup(t, group)

	if err := client.InstanceGroups.Delete(ctx, "group_123"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestInstanceGroupServiceUpdateOnlySendsProvidedFields(t *testing.T) {
	name := "renamed-group"
	sshKeyIDs := []string{"key_b"}
	tests := []struct {
		name     string
		req      UpdateInstanceGroupRequest
		wantBody string
	}{
		{
			name:     "name only",
			req:      UpdateInstanceGroupRequest{Name: &name},
			wantBody: `{"name":"renamed-group"}`,
		},
		{
			name: "template only",
			req: UpdateInstanceGroupRequest{Template: &UpdateInstanceGroupTemplateRequest{
				SSHKeyIDs: &sshKeyIDs,
			}},
			wantBody: `{"template":{"ssh_key_ids":["key_b"]}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockServer := testutil.NewMockServer()
			defer mockServer.Close()
			client := NewTestClient(t, mockServer)
			mockServer.SetHandler(http.MethodPatch, instanceGroupsBasePath+"/group_123", func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read update request: %v", err)
				}
				if string(body) != test.wantBody {
					t.Errorf("update body = %s, want %s", body, test.wantBody)
				}
				writeTestInstanceGroupResponse(t, w, http.StatusOK, testInstanceGroupResponseJSON)
			})

			group, err := client.InstanceGroups.Update(t.Context(), "group_123", test.req)
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
			if group.ID != "group_123" {
				t.Errorf("group ID = %q, want group_123", group.ID)
			}
		})
	}
}

func TestInstanceGroupServiceRejectsInvalidIDs(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	calls := []struct {
		name string
		call func(string) error
	}{
		{"get", func(id string) error { _, err := client.InstanceGroups.GetByID(t.Context(), id); return err }},
		{"update", func(id string) error {
			_, err := client.InstanceGroups.Update(t.Context(), id, UpdateInstanceGroupRequest{})
			return err
		}},
		{"delete", func(id string) error { return client.InstanceGroups.Delete(t.Context(), id) }},
	}
	for _, id := range []string{"", ".", ".."} {
		for _, call := range calls {
			t.Run(call.name+"/"+id, func(t *testing.T) {
				if err := call.call(id); err == nil || !strings.Contains(err.Error(), "instanceGroupID") {
					t.Errorf("error = %v, want instanceGroupID path error", err)
				}
			})
		}
	}
}

func TestInstanceGroupServiceValidatesBeforeRequest(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var requests atomic.Int32
	countRequest := func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		testutil.ErrorResponse(w, http.StatusInternalServerError, "unexpected request")
	}
	mockServer.SetHandler(http.MethodPost, instanceGroupsBasePath, countRequest)
	mockServer.SetHandler(http.MethodPatch, instanceGroupsBasePath+"/group_123", countRequest)

	if _, err := client.InstanceGroups.Create(t.Context(), CreateInstanceGroupRequest{}); err == nil {
		t.Error("Create with an empty request succeeded, want validation error")
	}
	emptyName := ""
	if _, err := client.InstanceGroups.Update(t.Context(), "group_123", UpdateInstanceGroupRequest{Name: &emptyName}); err == nil {
		t.Error("Update with an empty name succeeded, want validation error")
	}
	if _, err := client.InstanceGroups.Update(t.Context(), "group_123", UpdateInstanceGroupRequest{Capacity: &InstanceGroupCapacityRequest{}}); err == nil {
		t.Error("Update with an empty capacity succeeded, want validation error")
	}
	if _, err := client.InstanceGroups.Update(t.Context(), "group_123", UpdateInstanceGroupRequest{Template: &UpdateInstanceGroupTemplateRequest{}}); err == nil {
		t.Error("Update with an empty template succeeded, want validation error")
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("invalid requests reached HTTP %d times, want 0", got)
	}
}

func TestInstanceGroupServiceAPIErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			mockServer := testutil.NewMockServer()
			defer mockServer.Close()
			client := NewTestClient(t, mockServer)
			mockServer.SetHandler(http.MethodGet, instanceGroupsBasePath+"/group_123", func(w http.ResponseWriter, r *http.Request) {
				testutil.ErrorResponse(w, status, "instance group error")
			})

			_, err := client.InstanceGroups.GetByID(t.Context(), "group_123")
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T %v, want *APIError", err, err)
			}
			if apiErr.StatusCode != status {
				t.Errorf("status = %d, want %d", apiErr.StatusCode, status)
			}
		})
	}
}

func TestInstanceGroupValidation(t *testing.T) {
	valid := CreateInstanceGroupRequest{
		Name:         "workers",
		LocationCode: "FIN-02",
		InstanceType: "8V100.80S",
		Template:     InstanceGroupTemplateRequest{Image: "ubuntu-24.04"},
	}
	with := func(mut func(*CreateInstanceGroupRequest)) CreateInstanceGroupRequest {
		r := valid
		mut(&r)
		return r
	}
	zero, negative, one := 0, -1, 1
	zeroPointer, negativePointer, onePointer := &zero, &negative, &one
	empty := ""
	emptyPointer := &empty
	disabled := false
	emptySSHKeys := []string{}

	runValidateCases(t, []validateCase{
		{"create valid", valid, ""},
		{"create empty", CreateInstanceGroupRequest{}, "instance_type: cannot be blank; location_code: cannot be blank; name: cannot be blank; template: (image: cannot be blank.)."},
		{"create name too long", with(func(r *CreateInstanceGroupRequest) { r.Name = strings.Repeat("n", 65) }), "name: the length must be no more than 64."},
		{"create description too long", with(func(r *CreateInstanceGroupRequest) {
			description := strings.Repeat("d", 256)
			r.Description = &description
		}), "description: the length must be no more than 255."},
		{"create platform management reserved", with(func(r *CreateInstanceGroupRequest) { r.Management = InstanceGroupManagementPlatform }), "management: must be a valid value."},
		{"create interconnect invalid", with(func(r *CreateInstanceGroupRequest) {
			r.Interconnect = &InstanceGroupInterconnectRequest{Type: "ETHERNET"}
		}), "interconnect: (type: must be a valid value.)."},
		{"create interconnect empty", with(func(r *CreateInstanceGroupRequest) {
			r.Interconnect = &InstanceGroupInterconnectRequest{}
		}), "interconnect: (type: cannot be blank.)."},
		{"create template image required", with(func(r *CreateInstanceGroupRequest) { r.Template.Image = "" }), "template: (image: cannot be blank.)."},
		{"create os volume size required", with(func(r *CreateInstanceGroupRequest) {
			r.Template.OSVolume = &InstanceGroupOSVolume{}
		}), "template: (os_volume: (size: cannot be blank.).)."},
		{"create os volume size positive", with(func(r *CreateInstanceGroupRequest) {
			r.Template.OSVolume = &InstanceGroupOSVolume{Size: 1}
		}), ""},
		{"create startup script empty", with(func(r *CreateInstanceGroupRequest) {
			r.Template.StartupScriptID = &empty
		}), "template: (startup_script_id: cannot be blank.)."},
		{"create capacity empty", with(func(r *CreateInstanceGroupRequest) {
			r.Capacity = &InstanceGroupCapacityRequest{}
		}), "capacity: at least one field must be set."},
		{"create capacity negative", with(func(r *CreateInstanceGroupRequest) {
			r.Capacity = &InstanceGroupCapacityRequest{Reserved: &negative}
		}), "capacity: (reserved: must be no less than 0.)."},
		{"create capacity incoherent", with(func(r *CreateInstanceGroupRequest) {
			r.Capacity = &InstanceGroupCapacityRequest{Reserved: &one, MaxSize: &zeroPointer}
		}), "capacity: max_size must be greater than or equal to reserved and min_size."},
		{"capacity empty", InstanceGroupCapacityRequest{}, "at least one field must be set"},
		{"capacity explicit zeros", InstanceGroupCapacityRequest{Reserved: &zero, MinSize: &zeroPointer, MaxSize: &zeroPointer}, ""},
		{"capacity null bounds", InstanceGroupCapacityRequest{MinSize: new(*int), MaxSize: new(*int)}, ""},
		{"capacity explicit false", InstanceGroupCapacityRequest{EnforceMaxSize: &disabled}, ""},
		{"capacity min negative", InstanceGroupCapacityRequest{MinSize: &negativePointer}, "min_size: must be no less than 0."},
		{"capacity max negative", InstanceGroupCapacityRequest{MaxSize: &negativePointer}, "max_size: must be no less than 0."},
		{"capacity reserved above max", InstanceGroupCapacityRequest{Reserved: &one, MaxSize: &zeroPointer}, "max_size must be greater than or equal to reserved and min_size"},
		{"capacity min above max", InstanceGroupCapacityRequest{MinSize: &onePointer, MaxSize: &zeroPointer}, "max_size must be greater than or equal to reserved and min_size"},
		{"update empty is valid", UpdateInstanceGroupRequest{}, ""},
		{"update capacity empty", UpdateInstanceGroupRequest{Capacity: &InstanceGroupCapacityRequest{}}, "capacity: at least one field must be set."},
		{"update template empty", UpdateInstanceGroupRequest{Template: &UpdateInstanceGroupTemplateRequest{}}, "template: at least one field must be set."},
		{"update name empty", UpdateInstanceGroupRequest{Name: &empty}, "name: cannot be blank."},
		{"update description clear", UpdateInstanceGroupRequest{Description: new(*string)}, ""},
		{"update startup script clear", UpdateInstanceGroupRequest{Template: &UpdateInstanceGroupTemplateRequest{StartupScriptID: new(*string)}}, ""},
		{"update SSH keys clear", UpdateInstanceGroupRequest{Template: &UpdateInstanceGroupTemplateRequest{SSHKeyIDs: &emptySSHKeys}}, ""},
		{"update startup script empty", UpdateInstanceGroupRequest{Template: &UpdateInstanceGroupTemplateRequest{StartupScriptID: &emptyPointer}}, "template: (startup_script_id: cannot be blank.)."},
	})
}
