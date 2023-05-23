package mock

import (
	"context"

	"github.com/nats-io/nats.go"

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

// NatsClient mock client
type NatsClient struct {
	DoConnect   func() error
	DoClose     func() error
	DoSubscribe func(subject string) error
	DoListen    func() error
	DoAck       func(msg *nats.Msg) error
}

func (c *NatsClient) Connect() error {
	return c.DoConnect()
}

func (c *NatsClient) Close() error {
	return c.DoClose()
}

func (c *NatsClient) Subscribe(subject string) error {
	return c.DoSubscribe(subject)
}

func (c *NatsClient) Listen() error {
	return c.DoListen()
}

func (c *NatsClient) Ack(msg *nats.Msg) error {
	return c.DoAck(msg)
}
