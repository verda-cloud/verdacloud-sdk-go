package verda

import (
	"context"
)

type VolumeTypeService struct {
	client *Client
}

func (s *VolumeTypeService) GetAllVolumeTypes(ctx context.Context) ([]VolumeType, error) {
	volumeTypes, _, err := getRequest[[]VolumeType](ctx, s.client, "/volume-types")
	if err != nil {
		return nil, err
	}
	return volumeTypes, nil
}

// Deprecated: Use GetAllVolumeTypes
func (s *VolumeTypeService) Get(ctx context.Context) ([]VolumeType, error) {
	return s.GetAllVolumeTypes(ctx)
}
