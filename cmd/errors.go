package cmd

import "errors"

var (
	// ErrNATSURLRequired is returned when a NATS url is missing
	ErrNATSURLRequired = errors.New("nats url is required and cannot be empty")
	// ErrNATSAuthRequired is returned when a NATS auth method is missing
	ErrNATSAuthRequired = errors.New("nats creds are required and cannot be empty")
	// ErrHAProxyBaseConfigRequired is returned when the base HAProxy config is missing
	ErrHAProxyBaseConfigRequired = errors.New("base haproxy config is required and cannot be empty")
	// ErrLBAPIURLRequired is returned when the LB API url is missing
	ErrLBAPIURLRequired = errors.New("loadbalancer api url is required and cannot be empty")
)
