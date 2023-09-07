package mock

import (
	"context"
	"time"

	lbapi "go.infratographer.com/load-balancer-api/pkg/client"
)

// LBAPIClient mock client
type LBAPIClient struct {
	DoGetLoadBalancer func(ctx context.Context, id string) (*lbapi.LoadBalancer, error)
}

func (c LBAPIClient) GetLoadBalancer(ctx context.Context, id string) (*lbapi.LoadBalancer, error) {
	return c.DoGetLoadBalancer(ctx, id)
}

// DataplaneAPIClient mock client
type DataplaneAPIClient struct {
	DoPostConfig            func(ctx context.Context, config string) error
	DoCheckConfig           func(ctx context.Context, config string) error
	DoAPIIsReady            func(ctx context.Context) bool
	DoWaitForDataPlaneReady func(ctx context.Context, retries int, sleep time.Duration) error
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

func (c DataplaneAPIClient) WaitForDataPlaneReady(ctx context.Context, retries int, sleep time.Duration) error {
	return c.DoWaitForDataPlaneReady(ctx, retries, sleep)
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
