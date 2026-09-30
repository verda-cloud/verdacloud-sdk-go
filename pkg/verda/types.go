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

// types.go contains generic utility types that are not tied to any specific
// service domain. Domain-specific types (structs, constants, Validate methods)
// live in the corresponding *_types.go file for each service.
//
// Only add types here if they are used across multiple service domains and
// have no natural home in a single service file.

package verda

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
)

// FlexibleFloat is a custom type that can unmarshal both string and float64 values
type FlexibleFloat float64

// UnmarshalJSON implements json.Unmarshaler to handle both string and float64 inputs
func (f *FlexibleFloat) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as float64 first
	var floatVal float64
	if err := json.Unmarshal(data, &floatVal); err == nil {
		*f = FlexibleFloat(floatVal)
		return nil
	}

	// Try to unmarshal as string
	var strVal string
	if err := json.Unmarshal(data, &strVal); err != nil {
		return err
	}

	floatVal, err := strconv.ParseFloat(strVal, 64)
	if err != nil {
		return err
	}

	*f = FlexibleFloat(floatVal)
	return nil
}

// MarshalJSON implements json.Marshaler to always marshal as float64
func (f FlexibleFloat) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(f))
}

// Float64 returns the float64 value
func (f FlexibleFloat) Float64() float64 {
	return float64(f)
}

// IPAllocation is an IP assignment directive for instance creation:
// IPAllocAuto, IPAllocNone, or a literal IPv4 address.
type IPAllocation string

const (
	IPAllocAuto IPAllocation = "auto"
	IPAllocNone IPAllocation = "none"
)

// IPAllocIPv4 wraps a literal IPv4 address, validating its format.
func IPAllocIPv4(ip string) (IPAllocation, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return "", fmt.Errorf("invalid IPv4 address: %q", ip)
	}
	return IPAllocation(ip), nil
}

// Validate checks the value is "auto", "none", or a valid IPv4 address.
func (a IPAllocation) Validate() error {
	switch a {
	case IPAllocAuto, IPAllocNone:
		return nil
	}
	addr, err := netip.ParseAddr(string(a))
	if err != nil || !addr.Is4() {
		return fmt.Errorf("must be %q, %q, or a valid IPv4 address, got %q", IPAllocAuto, IPAllocNone, string(a))
	}
	return nil
}

func (a IPAllocation) String() string { return string(a) }
