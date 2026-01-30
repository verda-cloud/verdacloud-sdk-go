package verda

import (
	"context"
	"fmt"
	"net/url"
)

type ClusterService struct {
	client *Client
}

func (s *ClusterService) Get(ctx context.Context) ([]Cluster, error) {
	clusters, _, err := getRequest[[]Cluster](ctx, s.client, "/clusters")
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

func (s *ClusterService) GetByID(ctx context.Context, id string) (*Cluster, error) {
	path := fmt.Sprintf("/clusters/%s", id)

	cluster, _, err := getRequest[Cluster](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (s *ClusterService) Create(ctx context.Context, req CreateClusterRequest) (*CreateClusterResponse, error) {
	if req.LocationCode == "" {
		req.LocationCode = LocationFIN03
	}

	response, _, err := postRequest[CreateClusterResponse](ctx, s.client, "/clusters", req)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// Action performs cluster lifecycle operations
func (s *ClusterService) Action(ctx context.Context, idList any, action string) error {
	req := ClusterActionRequest{
		IDList: idList,
		Action: action,
	}

	_, _, err := putRequest[any](ctx, s.client, "/clusters", req)
	return err
}

// Boot boots a cluster
func (s *ClusterService) Boot(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ClusterActionBoot)
}

// Start starts a cluster
func (s *ClusterService) Start(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ClusterActionStart)
}

// Shutdown gracefully shuts down a cluster
func (s *ClusterService) Shutdown(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ClusterActionShutdown)
}

// ForceShutdown forcefully shuts down a cluster
func (s *ClusterService) ForceShutdown(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ClusterActionForceShutdown)
}

// Delete deletes a cluster
func (s *ClusterService) Delete(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ClusterActionDelete)
}

// DeleteStuck deletes a stuck cluster
func (s *ClusterService) DeleteStuck(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ClusterActionDeleteStuck)
}

// Discontinue discontinues a cluster
func (s *ClusterService) Discontinue(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ClusterActionDiscontinue)
}

// Hibernate hibernates a cluster - must be shut down first
func (s *ClusterService) Hibernate(ctx context.Context, idList any) error {
	return s.Action(ctx, idList, ClusterActionHibernate)
}

func (s *ClusterService) GetClusterTypes(ctx context.Context, currency string) ([]ClusterType, error) {
	path := "/cluster-types"

	if currency != "" {
		params := url.Values{}
		params.Set("currency", currency)
		path += "?" + params.Encode()
	}

	clusterTypes, _, err := getRequest[[]ClusterType](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return clusterTypes, nil
}

func (s *ClusterService) GetAvailabilities(ctx context.Context, locationCode string) ([]ClusterAvailability, error) {
	path := "/cluster-availability"

	if locationCode != "" {
		params := url.Values{}
		params.Set("location_code", locationCode)
		path += "?" + params.Encode()
	}

	availabilities, _, err := getRequest[[]ClusterAvailability](ctx, s.client, path)
	if err != nil {
		return nil, err
	}

	return availabilities, nil
}

func (s *ClusterService) CheckClusterTypeAvailability(ctx context.Context, clusterType string, locationCode string) (bool, error) {
	path := fmt.Sprintf("/cluster-availability/%s", clusterType)

	if locationCode != "" {
		params := url.Values{}
		params.Set("location_code", locationCode)
		path += "?" + params.Encode()
	}

	available, _, err := getRequest[bool](ctx, s.client, path)
	if err != nil {
		return false, err
	}

	return available, nil
}

func (s *ClusterService) GetImages(ctx context.Context) ([]ClusterImage, error) {
	images, _, err := getRequest[[]ClusterImage](ctx, s.client, "/images/cluster")
	if err != nil {
		return nil, err
	}

	return images, nil
}
