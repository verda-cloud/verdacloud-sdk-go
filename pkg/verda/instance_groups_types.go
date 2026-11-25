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
	"errors"
	"fmt"
	"time"

	validation "github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/internal/validation"
)

// Instance group management modes.
// Experimental: These values and the instance-group API may change in backwards-incompatible ways.
const (
	InstanceGroupManagementExternal = "EXTERNAL"
	// Platform management is reserved and is not accepted by the current create API.
	InstanceGroupManagementPlatform = "PLATFORM"
)

// Instance group interconnect types.
// Experimental: These values and the instance-group API may change in backwards-incompatible ways.
const (
	InstanceGroupInterconnectNone       = "NONE"
	InstanceGroupInterconnectInfiniBand = "INFINIBAND"
)

// Derived instance-group reservation states.
// Experimental: These values and the instance-group API may change in backwards-incompatible ways.
const (
	InstanceGroupReservationStateEmpty           = "empty"
	InstanceGroupReservationStateOccupied        = "occupied"
	InstanceGroupReservationStateOccupiedSpot    = "occupied_spot"
	InstanceGroupReservationStateSpotBackfill    = "spot_backfill"
	InstanceGroupReservationStateOccupiedForeign = "occupied_foreign"
)

// InstanceGroup represents an externally controlled instance group.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroup struct {
	ID           string                     `json:"id"`
	Name         string                     `json:"name"`
	Description  *string                    `json:"description"`
	ProjectID    string                     `json:"project_id"`
	LocationCode string                     `json:"location_code"`
	InstanceType string                     `json:"instance_type"`
	Management   string                     `json:"management"`
	Size         int                        `json:"size"`
	Interconnect InstanceGroupInterconnect  `json:"interconnect"`
	Template     InstanceGroupTemplate      `json:"template"`
	Capacity     InstanceGroupCapacity      `json:"capacity"`
	Reservations []InstanceGroupReservation `json:"reservations"`
	Instances    []InstanceGroupInstance    `json:"instances"`
	CreatedAt    time.Time                  `json:"created_at"`
	UpdatedAt    time.Time                  `json:"updated_at"`
	DeletedAt    *time.Time                 `json:"deleted_at"`
}

// InstanceGroupInterconnect describes a group's placement intent and island.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupInterconnect struct {
	Type   string  `json:"type"`
	Domain *string `json:"domain"`
}

// InstanceGroupTemplate contains the defaults inherited by group instances.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupTemplate struct {
	Image           string                 `json:"image"`
	OSVolume        *InstanceGroupOSVolume `json:"os_volume"`
	SSHKeyIDs       []string               `json:"ssh_key_ids"`
	StartupScriptID *string                `json:"startup_script_id"`
}

// InstanceGroupOSVolume configures the template's minimum OS-volume size.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupOSVolume struct {
	Size int `json:"size"`
}

func (r InstanceGroupOSVolume) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Size, validation.Required, validation.Min(1)),
	)
}

// InstanceGroupCapacity reports reserved and advisory instance capacity.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupCapacity struct {
	Reserved     int `json:"reserved"`
	ReservedFree int `json:"reserved_free"`
	// MinSize and MaxSize are nil for advisory null or absent bounds; zero is explicit.
	MinSize        *int `json:"min_size"`
	MaxSize        *int `json:"max_size"`
	EnforceMaxSize bool `json:"enforce_max_size"`
}

// InstanceGroupReservation reports derived occupancy without physical host identity.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupReservation struct {
	State      string  `json:"state"`
	InstanceID *string `json:"instance_id"`
}

// InstanceGroupInstance is a group member in an InstanceGroup response.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupInstance struct {
	ID         string `json:"id"`
	Hostname   string `json:"hostname"`
	Status     string `json:"status"`
	OnReserved bool   `json:"on_reserved"`
}

// CreateInstanceGroupRequest is the body of POST /instance-groups.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type CreateInstanceGroupRequest struct {
	Name         string                            `json:"name"`
	Description  *string                           `json:"description,omitempty"`
	LocationCode string                            `json:"location_code"`
	InstanceType string                            `json:"instance_type"`
	Management   string                            `json:"management,omitempty"`
	Interconnect *InstanceGroupInterconnectRequest `json:"interconnect,omitempty"`
	Template     InstanceGroupTemplateRequest      `json:"template"`
	Capacity     *InstanceGroupCapacityRequest     `json:"capacity,omitempty"`
}

func (r CreateInstanceGroupRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required, validation.RuneLength(0, 64)),
		validation.Field(&r.Description, validation.RuneLength(0, 255)),
		validation.Field(&r.LocationCode, validation.Required),
		validation.Field(&r.InstanceType, validation.Required),
		validation.Field(&r.Management, validation.In(InstanceGroupManagementExternal)),
		validation.Field(&r.Interconnect),
		validation.Field(&r.Template),
		validation.Field(&r.Capacity),
	)
}

// InstanceGroupInterconnectRequest selects the group's interconnect type.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupInterconnectRequest struct {
	Type string `json:"type"`
}

func (r InstanceGroupInterconnectRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Type, validation.Required, validation.In(
			InstanceGroupInterconnectNone,
			InstanceGroupInterconnectInfiniBand,
		)),
	)
}

// InstanceGroupTemplateRequest contains create-time instance defaults.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupTemplateRequest struct {
	Image           string                 `json:"image"`
	OSVolume        *InstanceGroupOSVolume `json:"os_volume,omitempty"`
	SSHKeyIDs       []string               `json:"ssh_key_ids,omitempty"`
	StartupScriptID *string                `json:"startup_script_id,omitempty"`
}

func (r InstanceGroupTemplateRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Image, validation.Required),
		validation.Field(&r.OSVolume),
		validation.Field(&r.StartupScriptID, validation.NilOrNotEmpty),
	)
}

// InstanceGroupCapacityRequest contains optional create-time or PATCH capacity values.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type InstanceGroupCapacityRequest struct {
	Reserved *int `json:"reserved,omitempty"`
	// Non-nil outer pointers holding nil send null for MinSize or MaxSize.
	MinSize        **int `json:"min_size,omitempty"`
	MaxSize        **int `json:"max_size,omitempty"`
	EnforceMaxSize *bool `json:"enforce_max_size,omitempty"`
}

func (r InstanceGroupCapacityRequest) Validate() error {
	if r.Reserved == nil && r.MinSize == nil && r.MaxSize == nil && r.EnforceMaxSize == nil {
		return errors.New("at least one field must be set")
	}

	if err := validation.ValidateStruct(&r,
		validation.Field(&r.Reserved, validation.Min(0)),
		validation.Field(&r.MinSize, validation.Min(0)),
		validation.Field(&r.MaxSize, validation.Min(0)),
	); err != nil {
		return err
	}

	if r.MaxSize == nil || *r.MaxSize == nil {
		return nil
	}
	maxSize := **r.MaxSize
	if r.Reserved != nil && *r.Reserved > maxSize ||
		r.MinSize != nil && *r.MinSize != nil && **r.MinSize > maxSize {
		return fmt.Errorf("max_size must be greater than or equal to reserved and min_size")
	}
	return nil
}

// UpdateInstanceGroupRequest contains mutable fields. Nil fields are left unchanged.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type UpdateInstanceGroupRequest struct {
	Name *string `json:"name,omitempty"`
	// A non-nil pointer to a nil string sends null to clear the description.
	Description **string                            `json:"description,omitempty"`
	Template    *UpdateInstanceGroupTemplateRequest `json:"template,omitempty"`
	Capacity    *InstanceGroupCapacityRequest       `json:"capacity,omitempty"`
}

func (r UpdateInstanceGroupRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.NilOrNotEmpty, validation.RuneLength(0, 64)),
		validation.Field(&r.Description, validation.RuneLength(0, 255)),
		validation.Field(&r.Template),
		validation.Field(&r.Capacity),
	)
}

// UpdateInstanceGroupTemplateRequest contains mutable instance defaults.
// Experimental: This API and its SDK types may change in backwards-incompatible ways.
type UpdateInstanceGroupTemplateRequest struct {
	// A pointer to an empty slice clears all default SSH keys.
	SSHKeyIDs *[]string `json:"ssh_key_ids,omitempty"`
	// A non-nil pointer to a nil string sends null to clear the startup script.
	StartupScriptID **string `json:"startup_script_id,omitempty"`
}

func (r UpdateInstanceGroupTemplateRequest) Validate() error {
	if r.SSHKeyIDs == nil && r.StartupScriptID == nil {
		return errors.New("at least one field must be set")
	}

	return validation.ValidateStruct(&r,
		validation.Field(&r.StartupScriptID, validation.NilOrNotEmpty),
	)
}
