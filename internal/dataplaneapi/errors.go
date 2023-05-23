package dataplaneapi

import "errors"

var (
	// ErrDataPlaneNotReady is returned dataplaneapi fails to return a 200
	ErrDataPlaneNotReady = errors.New("dataplaneapi failed to become ready")

	// ErrDataPlaneHTTPUnauthorized is returned when the request is not authorized
	ErrDataPlaneHTTPUnauthorized = errors.New("dataplaneapi received unauthorized request")

	// ErrDataPlaneHTTPError is returned when the http response is an error
	ErrDataPlaneHTTPError = errors.New("dataplaneapi http error")

	// ErrDataPlaneConfigInvalid is returned when the config is invalid
	ErrDataPlaneConfigInvalid = errors.New("dataplaneapi config is invalid")
)
