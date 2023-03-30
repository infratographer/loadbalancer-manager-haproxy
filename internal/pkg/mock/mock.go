package mock

import (
	"context"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/lbapi"
)

// LBAPIClient mock client
type LBAPIClient struct {
	DoGetLoadBalancer func(ctx context.Context, id string) (*lbapi.LoadBalancerResponse, error)
	DoGetPool         func(ctx context.Context, id string) (*lbapi.PoolResponse, error)
}

func (c LBAPIClient) GetLoadBalancer(ctx context.Context, id string) (*lbapi.LoadBalancerResponse, error) {
	return c.DoGetLoadBalancer(ctx, id)
}

func (c LBAPIClient) GetPool(ctx context.Context, id string) (*lbapi.PoolResponse, error) {
	return c.DoGetPool(ctx, id)
}

// DataplaneAPIClient mock client
type DataplaneAPIClient struct {
	DoPostConfig func(ctx context.Context, config string) error
	DoApiIsReady func(ctx context.Context) bool
}

func (c *DataplaneAPIClient) PostConfig(ctx context.Context, config string) error {
	return c.DoPostConfig(ctx, config)
}

func (c DataplaneAPIClient) ApiIsReady(ctx context.Context) bool {
	return c.DoApiIsReady(ctx)
}
