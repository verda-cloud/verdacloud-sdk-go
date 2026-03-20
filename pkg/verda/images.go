package verda

import "context"

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
	images, _, err := getRequest[[]Image](ctx, s.client, "/images")
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
