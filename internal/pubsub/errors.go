package pubsub

import "errors"

var (
	// ErrMsgHandlerNotRegistered is returned when the message handler callback is not registered
	ErrMsgHandlerNotRegistered = errors.New("nats message handler callback is not registered")
)
