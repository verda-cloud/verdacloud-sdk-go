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
	"fmt"
	"net/url"
)

// InstanceAvailability represents instance availability information
type InstanceAvailability struct {
	LocationCode   string   `json:"location_code"`
	Availabilities []string `json:"availabilities"`
}

// LocationAvailability represents instance type availability by location code
type LocationAvailability struct {
	LocationCode   string   `json:"location_code"`
	Availabilities []string `json:"availabilities"`
}

type InstanceAvailabilityService struct {
	client *Client
}

func (s *InstanceAvailabilityService) GetAllAvailabilities(ctx context.Context, isSpot bool, locationCode string) ([]LocationAvailability, error) {
	path := "/instance-availability"

	params := url.Values{}
	if isSpot {
		params.Set("is_spot", "true")
	}
	if locationCode != "" {
		params.Set("location_code", locationCode)
	}

	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	availabilities, _, err := getRequest[[]LocationAvailability](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return availabilities, nil
}

func (s *InstanceAvailabilityService) GetInstanceTypeAvailability(ctx context.Context, instanceType string, isSpot bool, locationCode string) (bool, error) {
	path := fmt.Sprintf("/instance-availability/%s", instanceType)

	params := url.Values{}
	if isSpot {
		params.Set("is_spot", "true")
	}
	if locationCode != "" {
		params.Set("location_code", locationCode)
	}

	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	available, _, err := getRequest[bool](ctx, s.client, path)
	if err != nil {
		return false, err
	}

	return available, nil
}
