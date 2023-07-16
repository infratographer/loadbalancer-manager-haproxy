package lbapi

import (
	"errors"
)

var (
	// ErrUnauthorized returned when the request is not authorized
	ErrUnauthorized = errors.New("client is unauthorized")

	// ErrNotfound returned when the load balancer ID not found
	ErrLBNotfound = errors.New("loadbalancer ID not found")

	// ErrLBHTTPError returned when the http response is an error
	ErrHTTPError = errors.New("loadbalancer api http error")
)
