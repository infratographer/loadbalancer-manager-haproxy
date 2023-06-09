package pubsub

import (
	"context"
	"sync"

	"github.com/ThreeDotsLabs/watermill/message"
	"go.infratographer.com/x/events"
	"go.uber.org/zap"
)

// MsgHandler is a callback function that processes messages delivered to subscribers
type MsgHandler func(msg *message.Message) error

// Subscriber is the subscriber client
type Subscriber struct {
	ctx            context.Context
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
func NewSubscriber(ctx context.Context, cfg events.SubscriberConfig, opts ...SubscriberOption) (*Subscriber, error) {
	sub, err := events.NewSubscriber(cfg)
	if err != nil {
		return nil, err
	}

	s := &Subscriber{
		ctx:        ctx,
		logger:     zap.NewNop().Sugar(),
		subscriber: sub,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.logger.Debugw("subscriber configuration", cfg)

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

// Listen start listening for messages on registered subjects and calls the registered message handler
func (s Subscriber) Listen() error {
	wg := &sync.WaitGroup{}

	if s.msgHandler == nil {
		return ErrMsgHandlerNotRegistered
	}

	// goroutine for each change channel
	for _, ch := range s.changeChannels {
		wg.Add(1)

		go s.listen(ch, wg)
	}

	wg.Wait()

	return nil
}

// listen listens for messages on a channel and calls the registered message handler
func (s Subscriber) listen(messages <-chan *message.Message, wg *sync.WaitGroup) {
	defer wg.Done()

	for msg := range messages {
		if err := s.msgHandler(msg); err != nil {
			s.logger.Warn("Failed to process msg: ", err)
		} else {
			msg.Ack()
		}
	}
}

// Close closes the nats connection and unsubscribes from all subscriptions
func (s *Subscriber) Close() error {
	return s.subscriber.Close()
}
