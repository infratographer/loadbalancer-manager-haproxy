package mock

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"

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
	DoAck       func(msg *message.Message) error
	DoNack      func(msg *message.Message) error
}

func (c *Subscriber) Close() error {
	return c.DoClose()
}

func (c *Subscriber) Subscribe(subject string) error {
	return c.DoSubscribe(subject)
}

func (c *Subscriber) Listen() error {
	return c.DoListen()
}

func (c *Subscriber) Ack(msg *message.Message) error {
	return c.DoAck(msg)
}

func (c *Subscriber) Nack(msg *message.Message) error {
	return c.DoNack(msg)
}
