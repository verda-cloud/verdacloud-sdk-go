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

const (
	IMAGE_PATH         = "/images"
	CLUSTER_IMAGE_PATH = "/images/cluster"
)

// Image represents an OS image for instances
type Image struct {
	ID        string   `json:"id"`
	ImageType string   `json:"image_type"`
	Name      string   `json:"name"`
	IsDefault bool     `json:"is_default"`
	IsCluster bool     `json:"is_cluster"`
	Details   []string `json:"details"`
	Category  string   `json:"category"`
}

type ImagesService struct {
	client *Client
}

func (s *ImagesService) Get(ctx context.Context) ([]Image, error) {
	images, _, err := getRequest[[]Image](ctx, s.client, IMAGE_PATH)
	if err != nil {
		return nil, err
	}
	return images, nil
}

// GetImagesByInstanceType lists OS images filtered by instance type.
// Pass a value such as "8B300.240V" to filter by instance type (?instance_type=...).
func (s *ImagesService) GetImagesByInstanceType(ctx context.Context, instanceType string) ([]Image, error) {
	path := IMAGE_PATH
	if instanceType != "" {
		params := url.Values{}
		params.Set("instance_type", instanceType)
		path += "?" + params.Encode()
	}

	images, _, err := getRequest[[]Image](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func (s *ImagesService) GetClusterImages(ctx context.Context) ([]ClusterImage, error) {
	images, _, err := getRequest[[]ClusterImage](ctx, s.client, CLUSTER_IMAGE_PATH)
	if err != nil {
		return nil, err
	}
	return images, nil
}
