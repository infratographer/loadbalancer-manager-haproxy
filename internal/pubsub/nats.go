package pubsub

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// MsgHandler is a callback function that processes messages delivered to subscribers
type MsgHandler func(msg *nats.Msg) error

// NatsClient is the nats client
type NatsClient struct {
	ctx           context.Context
	url           string
	conn          *nats.Conn
	userCreds     string
	msgBus        chan *nats.Msg
	subscriptions []*nats.Subscription
	msgHandler    MsgHandler
	logger        *zap.SugaredLogger
}

// NatsOption is a functional option for the NatsClient
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

// WithMsgHandler sets the message handler callback for the NatsClient
func WithMsgHandler(cb MsgHandler) NatsOption {
	return func(c *NatsClient) {
		c.msgHandler = cb
	}
}

// NewNatsClient creates a new NatsClient
func NewNatsClient(ctx context.Context, url string, opts ...NatsOption) *NatsClient {
	c := &NatsClient{
		ctx:    ctx,
		url:    url,
		msgBus: make(chan *nats.Msg),
		logger: zap.NewNop().Sugar(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Connect connects to the nats server
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

	s, err := c.conn.ChanSubscribe(subject, c.msgBus)
	if err != nil {
		return err
	}

	c.subscriptions = append(c.subscriptions, s)

	return nil
}

// Ack acknowledges a nats message
func (c *NatsClient) Ack(msg *nats.Msg) error {
	return msg.Ack()
}

// Listen start listening for messages on registered subjects and calls the registered message handler
func (c *NatsClient) Listen() error {
	if c.conn == nil || c.conn.IsClosed() {
		return ErrNatsConnClosed
	}

	if c.msgHandler == nil {
		return ErrMsgHandlerNotRegistered
	}

	for {
		select {
		case <-c.ctx.Done():
			return nil
		case msg := <-c.msgBus:
			if err := c.msgHandler(msg); err != nil {
				c.logger.Warn("Failed to process msg: ", err)
			}
		}
	}
}

// Close closes the nats connection and unsubscribes from all subscriptions
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
