package lbapi

import (
	"errors"
)

var (
	// ErrLBHTTPUnauthorized is returned when the request is not authorized
	ErrLBHTTPUnauthorized = errors.New("load balancer api received unauthorized request")

	// ErrLBHTTPNotfound is returned when the load balancer ID not found
	ErrLBHTTPNotfound = errors.New("load balancer ID not found")

	// ErrLBHTTPError is returned when the http response is an error
	ErrLBHTTPError = errors.New("load balancer api http error")
)
