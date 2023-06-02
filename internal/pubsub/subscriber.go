package pubsub

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	"go.infratographer.com/x/events"
	"go.uber.org/zap"
)

// MsgHandler is a callback function that processes messages delivered to subscribers
type MsgHandler func(msg *message.Message) error

// Subscriber is the subscriber client
type Subscriber struct {
	ctx            context.Context
	url            string
	changeChannels []<-chan *message.Message
	msgHandler     MsgHandler
	logger         *zap.SugaredLogger
	subscriber     *events.Subscriber
}

// SubscriberOption is a functional option for the Subscriber
type SubscriberOption func(s *Subscriber)

// WithLogger sets the logger for the Subscriber
func WithLogger(l *zap.SugaredLogger) SubscriberOption {
	return func(s *Subscriber) {
		s.logger = l
	}
}

// WithMsgHandler sets the message handler callback for the Subscriber
func WithMsgHandler(cb MsgHandler) SubscriberOption {
	return func(s *Subscriber) {
		s.msgHandler = cb
	}
}

// NewSubscriber creates a new Subscriber
func NewSubscriber(ctx context.Context, url string, cfg events.SubscriberConfig, opts ...SubscriberOption) (*Subscriber, error) {
	sub, err := events.NewSubscriber(cfg)
	if err != nil {
		return nil, err
	}

	s := &Subscriber{
		ctx:        ctx,
		url:        url,
		logger:     zap.NewNop().Sugar(),
		subscriber: sub,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Subscribe subscribes to a nats subject
func (s *Subscriber) Subscribe(topic string) error {
	msgChan, err := s.subscriber.SubscribeChanges(s.ctx, topic)
	if err != nil {
		return err
	}

	s.changeChannels = append(s.changeChannels, msgChan)

	return nil
}

// Ack acknowledges a message
func (s *Subscriber) Ack(msg *message.Message) {
	msg.Ack()
}

// Nack negative acknowledges a message
func (s *Subscriber) Nack(msg *message.Message) {
	msg.Nack()
}

// Listen start listening for messages on registered subjects and calls the registered message handler
func (s *Subscriber) Listen() error {
	if s.msgHandler == nil {
		return ErrMsgHandlerNotRegistered
	}

	// goroutine for each change channel
	for _, ch := range s.changeChannels {
		go s.listen(ch)
	}

	return nil
}

// listen listens for messages on a channel and calls the registered message handler
func (s Subscriber) listen(ch <-chan *message.Message) {
	for msg := range ch {
		if err := s.msgHandler(msg); err != nil {
			s.logger.Warn("Failed to process msg: ", err)
		}
	}
}

// Close closes the nats connection and unsubscribes from all subscriptions
func (s *Subscriber) Close() error {
	// TODO: once @tyler's PR is merged, return this
	// return s.subscriber.Close()
	return nil
}
