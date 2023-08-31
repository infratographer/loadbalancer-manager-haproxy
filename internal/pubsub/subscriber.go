package pubsub

import (
	"context"
	"sync"
	"time"

	"go.infratographer.com/x/events"
	"go.uber.org/zap"
)

const defaultNakDelay = 10 * time.Second

// MsgHandler is a callback function that processes messages delivered to subscribers
type MsgHandler func(msg events.Message[events.ChangeMessage]) error

// Subscriber is the subscriber client
type Subscriber struct {
	ctx                   context.Context
	changeChannels        []<-chan events.Message[events.ChangeMessage]
	msgHandler            MsgHandler
	logger                *zap.SugaredLogger
	connection            events.Connection
	maxProcessMsgAttempts uint64
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

// WithMaxMsgProcessAttempts sets the maximum number of times a message will attempt to process before being terminated
func WithMaxMsgProcessAttempts(max uint64) SubscriberOption {
	return func(s *Subscriber) {
		s.maxProcessMsgAttempts = max
	}
}

// NewSubscriber creates a new Subscriber
func NewSubscriber(ctx context.Context, connection events.Connection, opts ...SubscriberOption) *Subscriber {
	s := &Subscriber{
		ctx:        ctx,
		logger:     zap.NewNop().Sugar(),
		connection: connection,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Subscribe subscribes to a nats subject
func (s *Subscriber) Subscribe(topic string) error {
	s.logger.Debugw("Subscribing to topic", "topic", topic)

	msgChan, err := s.connection.SubscribeChanges(s.ctx, topic)
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
func (s Subscriber) listen(messages <-chan events.Message[events.ChangeMessage], wg *sync.WaitGroup) {
	defer wg.Done()

	for msg := range messages {
		slogger := s.logger.With(
			"event.message.id", msg.ID(),
			"event.message.topic", msg.Topic(),
			"event.message.source", msg.Source(),
			"event.message.timestamp", msg.Timestamp(),
			"event.message.deliveries", msg.Deliveries(),
		)

		if err := s.msgHandler(msg); err != nil {
			if s.maxProcessMsgAttempts != 0 && msg.Deliveries()+1 > s.maxProcessMsgAttempts {
				slogger.Warnw("terminating event, too many attempts")

				if termErr := msg.Term(); termErr != nil {
					slogger.Warnw("error occurred while terminating event")
				}
			} else if nakErr := msg.Nak(defaultNakDelay); nakErr != nil {
				slogger.Warnw("error occurred while naking", "error", nakErr)
			}
		} else if ackErr := msg.Ack(); ackErr != nil {
			slogger.Warnw("error occurred while acking", "error", ackErr)
		}
	}
}
