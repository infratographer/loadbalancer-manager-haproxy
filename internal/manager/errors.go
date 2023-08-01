package manager

import (
	"errors"
	"fmt"
)

var (
	// errLoadBalancerIDParamInvalid is returned when an invalid load balancer ID is provided
	errLoadBalancerIDParamInvalid = errors.New("loadbalancer ID is empty")

	// errFrontendSectionLabelFailure is returned when a frontend section cannot be created
	errFrontendSectionLabelFailure = errors.New("failed to create frontend section with label")

	// errUseBackendFailure is returned when the use_backend attr cannot be applied to a frontend
	errUseBackendFailure = errors.New("failed to create frontend attr use_backend")

	// errFrontendBindFailure is returned when the bind attribute cannot be applied to a frontend
	errFrontendBindFailure = errors.New("failed to create frontend attr bind")

	// errBackendSectionLabelFailure is returned when a backend section cannot be created
	errBackendSectionLabelFailure = errors.New("failed to create section backend with label")

	// errBackendServerFailure is returned when a server cannot be applied to a backend
	errBackendServerFailure = errors.New("failed to add backend attr server: ")
)

func newLabelError(label string, err error, labelErr error) error {
	return fmt.Errorf("%w %q: %v", err, label, labelErr)
}

func newAttrError(err error, attrErr error) error {
	return fmt.Errorf("%w: %v", err, attrErr)
}
