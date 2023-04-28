package lbapi

import (
	"errors"
	"fmt"
)

var (
	// errDecodeLB is returned when the client is unable to decode the loadbalancer response
	errDecodeLB = errors.New("failed to decode load balancer")

	// errReadResponse is returned when the client is unable to read the response body
	errReadResponse = errors.New("failed to read response body")

	// ErrLBHTTPUnauthorized is returned when the request is not authorized
	ErrLBHTTPUnauthorized = errors.New("load balancer api received unauthorized request")

	// ErrLBHTTPNotfound is returned when the load balancer ID not found
	ErrLBHTTPNotfound = errors.New("load balancer ID not found")

	// ErrLBHTTPError is returned when the http response is an error
	ErrLBHTTPError = errors.New("load balancer api http error")
)

func newError(err error, subErr error) error {
	return fmt.Errorf("%w: %v", err, subErr)
}
