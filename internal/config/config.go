// Package config provides a struct to stoire the application's configuration
package config

import (
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/loggingx"

	"go.infratographer.com/x/oauth2x"
)

// EventsConfig stores the configuration for a load-balancer-api events config
type EventsConfig struct {
	Subscriber events.SubscriberConfig
}

// OIDCClientConfig stores the configuration for an OIDC client
type OIDCClientConfig struct {
	Client oauth2x.Config
}

var AppConfig struct {
	Events  EventsConfig
	Logging loggingx.Config
	OIDC    OIDCClientConfig
}
