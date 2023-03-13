// Package pubsub provides NATS
package pubsub

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"gocloud.dev/pubsub"
	"gocloud.dev/pubsub/natspubsub"
)

// LoadBalancerMessage is the NATS message from Load Balancer API
type LoadBalancerMessage struct {
	LoadBalancerID uuid.UUID `json:"load_balancer_id"`
}

// NATSClient is a NATS client with some configuration
type NATSClient struct {
	Conn     *nats.Conn
	Messages chan *pubsub.Message
	logger   *zap.SugaredLogger
}

// NATSOption is a functional configuration option for NATS
type NATSOption func(c *NATSClient)

// NewNATSClient configures and establishes a new NATS client connection
func NewNATSClient(opts ...NATSOption) *NATSClient {
	client := NATSClient{
		Messages: make(chan *pubsub.Message),
		logger:   zap.NewNop().Sugar(),
	}

	for _, opt := range opts {
		opt(&client)
	}

	return &client
}

// WithNATSConn sets the nats connection
func WithNATSConn(nc *nats.Conn) NATSOption {
	return func(c *NATSClient) {
		c.Conn = nc
	}
}

// WithNATSLogger sets the NATS client logger
func WithNATSLogger(l *zap.SugaredLogger) NATSOption {
	return func(c *NATSClient) {
		c.logger = l
	}
}

// Listen opens a subscription for a NATS subject and receives messages
func (n *NATSClient) Listen(ctx context.Context, subject string) {
	subscription, err := natspubsub.OpenSubscription(n.Conn, subject, nil)
	if err != nil {
		n.logger.Error("failed to subscribe to NATS subject ", "subject: ", subject)
		return
	}

	n.logger.Info("subscribed to NATS subject ", "subject: ", subject)

	defer func() {
		n.logger.Info("shutting down NATS subscription ", "subject: ", subject)

		_ = subscription.Shutdown(ctx)
	}()

	for {
		msg, err := subscription.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				n.logger.Info("context canceled")
				return
			}

			n.logger.Error(err)

			return
		}

		n.Messages <- msg
	}
}
