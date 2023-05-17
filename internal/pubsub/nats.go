package pubsub

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type NatsClient struct {
	ctx           context.Context
	url           string
	conn          *nats.Conn
	userCreds     string
	MsgBus        chan *nats.Msg
	subscriptions []*nats.Subscription
	logger        *zap.SugaredLogger
}

type NatsOption func(c *NatsClient)

// WithLogger sets the logger for the NatsClient
func WithLogger(l *zap.SugaredLogger) NatsOption {
	return func(c *NatsClient) {
		c.logger = l
	}
}

// WithUserCredentials sets the user credentials for the NatsClient
func WithUserCredentials(creds string) NatsOption {
	return func(c *NatsClient) {
		c.userCreds = creds
	}
}

// NewNatsClient creates a new NatsClient
func NewNatsClient(ctx context.Context, url string, opts ...NatsOption) *NatsClient {
	c := &NatsClient{
		ctx:    ctx,
		url:    url,
		MsgBus: make(chan *nats.Msg),
		logger: zap.NewNop().Sugar(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *NatsClient) Connect() error {
	conn, err := nats.Connect(c.url, nats.UserCredentials(c.userCreds))
	if err != nil {
		return err
	}

	c.conn = conn

	return nil
}

// Subscribe subscribes to a nats subject
func (c *NatsClient) Subscribe(subject string) error {
	if c.conn == nil || c.conn.IsClosed() {
		return ErrNatsConnClosed
	}

	s, err := c.conn.ChanSubscribe(subject, c.MsgBus)
	if err != nil {
		return err
	}

	c.subscriptions = append(c.subscriptions, s)

	return nil
}

func (c *NatsClient) Close() error {
	c.logger.Info("Unsubscribing from nats subscriptions")

	for _, sub := range c.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	if c.conn != nil && !c.conn.IsClosed() {
		c.logger.Info("Shutting down nats connection")
		c.conn.Close()
	}

	return nil
}
