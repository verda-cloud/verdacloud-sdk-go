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
)

// LongTermPeriod represents a long-term rental period option
type LongTermPeriod struct {
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	IsEnabled          bool    `json:"is_enabled"`
	UnitName           string  `json:"unit_name"`
	UnitValue          int     `json:"unit_value"`
	DiscountPercentage float64 `json:"discount_percentage"`
}

type LongTermService struct {
	client *Client
}

func (s *LongTermService) GetInstancePeriods(ctx context.Context) ([]LongTermPeriod, error) {
	path := "/long-term/periods/instances"

	periods, _, err := getRequest[[]LongTermPeriod](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return periods, nil
}

func (s *LongTermService) GetPeriods(ctx context.Context) ([]LongTermPeriod, error) {
	path := "/long-term/periods"

	periods, _, err := getRequest[[]LongTermPeriod](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return periods, nil
}

func (s *LongTermService) GetClusterPeriods(ctx context.Context) ([]LongTermPeriod, error) {
	path := "/long-term/periods/clusters"

	periods, _, err := getRequest[[]LongTermPeriod](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return periods, nil
}
