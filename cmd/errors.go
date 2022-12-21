package cmd

import "errors"

// TODO: Add your error definitions here
var (
	// ErrNATSURLRequired is returned when a NATS url is missing
	ErrNATSURLRequired = errors.New("nats url is required and cannot be empty")
	// ErrNATSAuthRequired is returned when a NATS auth method is missing
	ErrNATSAuthRequired = errors.New("nats token or nkey auth is required and cannot be empty")
	// ErrNATSTokenAuthForDev is returned if token auth is requested, but the service is not in dev mode
	ErrNATSTokenAuthForDev = errors.New("nats token auth is for development environments only")
)
