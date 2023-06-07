package cmd

import "errors"

var (
	// ErrSubscriberURLRequired is returned when a event subscriber url is missing
	ErrSubscriberURLRequired = errors.New("nats url is required and cannot be empty")

	// ErrSubscriberPrefixRequired is returned when a NATS subject prefix is missing
	ErrSubscriberPrefixRequired = errors.New("env LOADBALANCER_MANAGER_HAPROXY_EVENTS_SUBSCRIBER_PREFIX is required and cannot be empty")

	ErrSubscriberTopicsRequired = errors.New("event-topics is required and cannot be empty")

	// ErrNATSAuthRequired is returned when a NATS auth method is missing
	ErrNATSAuthRequired = errors.New("env LOADBALANCER_MANAGER_HAPROXY_EVENTS_SUBSCRIBER_NATS_CREDSFILE is required and cannot be empty")

	// ErrHAProxyBaseConfigRequired is returned when the base HAProxy config is missing
	ErrHAProxyBaseConfigRequired = errors.New("base-haproxy-config is required and cannot be empty")

	// ErrLBAPIURLRequired is returned when the LB API url is missing
	ErrLBAPIURLRequired = errors.New("loadbalancer-api-url is required and cannot be empty")

	// ErrLBIDRequired is the loadbalancer id to watch for changes on the msg queue
	ErrLBIDRequired = errors.New("loadbalancer-id is required and cannot be empty")

	// ErrLBIDInvalid is returned when the loadbalancer gidx is invalid
	ErrLBIDInvalid = errors.New("loadbalancer-id (gidx) is invalid")
)
