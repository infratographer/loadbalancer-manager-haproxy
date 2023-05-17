package pubsub

import "errors"

var (
	// ErrNatsConnClosed is returned when the nats connection is closed
	// and you are trying to modify the connection
	ErrNatsConnClosed = errors.New("nats connection is closed")
)
