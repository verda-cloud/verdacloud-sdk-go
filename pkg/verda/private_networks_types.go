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
	"fmt"
	"net/netip"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Private network modes: "auto" creates subnets per location on first use,
// "custom" requires creating them explicitly.
const (
	NetworkModeAuto   = "auto"
	NetworkModeCustom = "custom"
)

// PrivateNetwork represents a private network. Networks are global;
// their subnets are per-location.
type PrivateNetwork struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Mode            string    `json:"mode"`
	IsDefault       bool      `json:"is_default"`
	CreatedByUserID string    `json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// CreatePrivateNetworkRequest represents the request to create a private network
type CreatePrivateNetworkRequest struct {
	Name      string `json:"name"`
	Mode      string `json:"mode,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`
}

// Validate validates the CreatePrivateNetworkRequest fields
func (r CreatePrivateNetworkRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Mode, validation.In(NetworkModeAuto, NetworkModeCustom)),
	)
}

// UpdatePrivateNetworkRequest renames a network or marks it default.
// Nil fields are left unchanged.
type UpdatePrivateNetworkRequest struct {
	Name      *string `json:"name,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`
}

// Validate validates the UpdatePrivateNetworkRequest fields
func (r UpdatePrivateNetworkRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.NilOrNotEmpty),
	)
}

// AttachedInstance is the instance a subnet's default route or a route points at
type AttachedInstance struct {
	ID        string  `json:"id"`
	Hostname  string  `json:"hostname"`
	PrivateIP string  `json:"private_ip"`
	PublicIP  *string `json:"public_ip"`
	Status    string  `json:"status"`
}

// Subnet represents a subnet within a private network
type Subnet struct {
	ID                   string            `json:"id"`
	Location             Location          `json:"location"`
	Name                 string            `json:"name"`
	CIDR                 string            `json:"cidr"`
	IsDefault            bool              `json:"is_default"`
	UsedIPCount          int               `json:"used_ip_count"`
	TotalIPCount         int               `json:"total_ip_count"`
	DefaultRouteInstance *AttachedInstance `json:"default_route_instance"`
	CreatedByUserID      string            `json:"created_by_user_id"`
	CreatedAt            time.Time         `json:"created_at"`
}

// CreateSubnetRequest represents the request to create a subnet
type CreateSubnetRequest struct {
	LocationCode string `json:"location_code"`
	Name         string `json:"name"`
	CIDR         string `json:"cidr"`
	IsDefault    bool   `json:"is_default,omitempty"`
}

// Validate validates the CreateSubnetRequest fields
func (r CreateSubnetRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.LocationCode, validation.Required),
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.CIDR, validation.Required, validation.By(validateIPv4CIDR)),
	)
}

// UpdateSubnetRequest renames a subnet or marks it its datacenter's default.
// The CIDR cannot change. Nil fields are left unchanged.
type UpdateSubnetRequest struct {
	Name      *string `json:"name,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`
}

// Validate validates the UpdateSubnetRequest fields
func (r UpdateSubnetRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.NilOrNotEmpty),
	)
}

// Route represents a route steering a subnet's traffic through an instance
type Route struct {
	ID              string            `json:"id"`
	SubnetID        string            `json:"subnet_id"`
	Location        Location          `json:"location"`
	Instance        *AttachedInstance `json:"instance"`
	Destination     string            `json:"destination"`
	CreatedByUserID string            `json:"created_by_user_id"`
	CreatedAt       time.Time         `json:"created_at"`
}

// CreateRouteRequest represents the request to route a subnet's traffic
// through one of the network's instances. The instance must be in the subnet's
// datacenter and have a public IP. Re-sending an existing destination moves it
// to another instance.
type CreateRouteRequest struct {
	Destination string `json:"destination"`
	InstanceID  string `json:"instance_id"`
}

// Validate validates the CreateRouteRequest fields
func (r CreateRouteRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Destination, validation.Required, validation.By(validateIPv4CIDR)),
		validation.Field(&r.InstanceID, validation.Required),
	)
}

func validateIPv4CIDR(value interface{}) error {
	s, _ := value.(string)
	prefix, err := netip.ParsePrefix(s)
	if err != nil || !prefix.Addr().Is4() {
		return fmt.Errorf("must be a valid IPv4 CIDR, got %q", s)
	}
	return nil
}
