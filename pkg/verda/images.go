package verda

import (
	"context"
	"net/url"
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

// Get lists OS images. Pass empty instanceType to omit the query parameter, or a value such as
// "8B300.240V" to filter by instance type (?instance_type=...).
func (s *ImagesService) Get(ctx context.Context, instanceType string) ([]Image, error) {
	path := "/images"
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
	images, _, err := getRequest[[]ClusterImage](ctx, s.client, "/images/cluster")
	if err != nil {
		return nil, err
	}
	return images, nil
}
