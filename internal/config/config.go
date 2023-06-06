// Package config provides a struct to stoire the application's configuration
package config

import (
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/loggingx"
)

// EventsConfig stores the configuration for a load-balancer-api events config
type EventsConfig struct {
	Subscriber events.SubscriberConfig
}

var AppConfig struct {
	Events  EventsConfig
	Logging loggingx.Config
}
