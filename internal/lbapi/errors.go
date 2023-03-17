package lbapi

import "errors"

var (
	// ErrLBHTTPUnauthorized is returned when the request is not authorized
	ErrLBHTTPUnauthorized = errors.New("load balancer api received unauthorized request")
	// ErrLBHTTPError is returned when the http response is an error
	ErrLBHTTPError = errors.New("load balancer api http error")
)
