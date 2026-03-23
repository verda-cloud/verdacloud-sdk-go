package verda

import (
	"context"
)

// VolumeType represents available volume type specifications
type VolumeType struct {
	Type                 string          `json:"type"`
	Price                VolumeTypePrice `json:"price"`
	IsSharedFS           bool            `json:"is_shared_fs"`
	BurstBandwidth       float64         `json:"burst_bandwidth"`
	ContinuousBandwidth  float64         `json:"continuous_bandwidth"`
	InternalNetworkSpeed float64         `json:"internal_network_speed"`
	IOPS                 string          `json:"iops"`
}

// VolumeTypePrice represents the pricing structure for a volume type
type VolumeTypePrice struct {
	MonthlyPerGB float64 `json:"monthly_per_gb"`
	Currency     string  `json:"currency"`
}

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
