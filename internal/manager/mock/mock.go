package mock

import (
	"context"

	"go.infratographer.com/loadbalancer-manager-haproxy/pkg/lbapi"
)

// LBAPIClient mock client
type LBAPIClient struct {
	DoGetLoadBalancer func(ctx context.Context, id string) (*lbapi.GetLoadBalancer, error)
}

func (c LBAPIClient) GetLoadBalancer(ctx context.Context, id string) (*lbapi.GetLoadBalancer, error) {
	return c.DoGetLoadBalancer(ctx, id)
}

// DataplaneAPIClient mock client
type DataplaneAPIClient struct {
	DoPostConfig  func(ctx context.Context, config string) error
	DoCheckConfig func(ctx context.Context, config string) error
	DoAPIIsReady  func(ctx context.Context) bool
}

func (c *DataplaneAPIClient) PostConfig(ctx context.Context, config string) error {
	return c.DoPostConfig(ctx, config)
}

func (c DataplaneAPIClient) APIIsReady(ctx context.Context) bool {
	return c.DoAPIIsReady(ctx)
}

func (c DataplaneAPIClient) CheckConfig(ctx context.Context, config string) error {
	return c.DoCheckConfig(ctx, config)
}

// Subscriber mock client
type Subscriber struct {
	DoClose     func() error
	DoSubscribe func(subject string) error
	DoListen    func() error
}

func (s *Subscriber) Close() error {
	return s.DoClose()
}

func (s *Subscriber) Subscribe(subject string) error {
	return s.DoSubscribe(subject)
}

func (s *Subscriber) Listen() error {
	return s.DoListen()
}
