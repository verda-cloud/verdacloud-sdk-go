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
	"net/url"
)

// ContainerType represents a serverless container compute resource option
type ContainerType struct {
	ID                  string         `json:"id"`
	Model               string         `json:"model"`
	Name                string         `json:"name"`
	InstanceType        string         `json:"instance_type"`
	CPU                 InstanceCPU    `json:"cpu"`
	GPU                 InstanceGPU    `json:"gpu"`
	GPUMemory           InstanceMemory `json:"gpu_memory"`
	Memory              InstanceMemory `json:"memory"`
	ServerlessPrice     FlexibleFloat  `json:"serverless_price"`
	ServerlessSpotPrice FlexibleFloat  `json:"serverless_spot_price"`
	Currency            string         `json:"currency"`
	Manufacturer        string         `json:"manufacturer"`
}

type ContainerTypesService struct {
	client *Client
}

func (s *ContainerTypesService) Get(ctx context.Context, currency string) ([]ContainerType, error) {
	path := "/container-types"

	if currency != "" {
		params := url.Values{}
		params.Set("currency", currency)
		path += "?" + params.Encode()
	}

	containerTypes, _, err := getRequest[[]ContainerType](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return containerTypes, nil
}
