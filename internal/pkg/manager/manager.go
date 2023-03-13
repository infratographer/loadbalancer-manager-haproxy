// Package manager handles configuration updates and initializes NATS listener
package manager

import (
	"context"
	"sync"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/spf13/viper"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pkg/dataplaneapi"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pkg/pubsub"
	"go.uber.org/zap"
	gopubsub "gocloud.dev/pubsub"
)

// Config contains configuration and client connections
type Config struct {
	Context         context.Context
	Logger          *zap.SugaredLogger
	NATSClient      *pubsub.NATSClient
	DataPlaneClient *dataplaneapi.Client
}

// Run subscribes to a NATS subject and updates the haproxy config via dataplaneapi
func (m *Config) Run() error {
	// wait until the Data Plane API is running
	if err := m.DataPlaneClient.WaitForDataPlaneReady(m.Context); err != nil {
		m.Logger.Fatal("unable to reach dataplaneapi. is it running?")
		return err
	}

	// use desired config on start
	if err := m.updateConfigToLatest(); err != nil {
		m.Logger.Error("failed to update the config", "error", err)
	}

	var wg sync.WaitGroup

	// start listening on all NATS subjects
	for _, s := range viper.GetStringSlice("nats.subjects") {
		// message listener
		wg.Add(1)

		go func(s string) {
			defer wg.Done()
			m.NATSClient.Listen(m.Context, s)
		}(s)
	}

	// message receiver
	wg.Add(1)

	go func() {
		defer wg.Done()
		m.receiveMessages()
	}()

	wg.Wait()

	return nil
}

func (m *Config) receiveMessages() {
	for {
		select {
		case <-m.Context.Done():
			return
		case msg := <-m.NATSClient.Messages:
			m.processMessage(msg)
		}
	}
}

func (m *Config) processMessage(msg *gopubsub.Message) {
	m.Logger.Debug("processing message ", "msg id: ", msg.LoggableID)

	// TODO - check message type and ID to see if message is intended for this lb

	// Ack messages with matching load balancer ID
	msg.Ack()

	m.updateConfigToLatest()
}

func (m *Config) updateConfigToLatest() error {
	m.Logger.Info("updating the config")

	// TODO: get load balancer object from lbapi, pass to buildConfig()

	cfg, err := m.buildConfig()
	if err != nil {
		return err
	}

	// post dataplaneapi
	if err = m.DataPlaneClient.PostConfig(m.Context, cfg.String()); err != nil {
		m.Logger.Error("failed to post new haproxy config", "error", err)
	}

	return err
}

func (m *Config) buildConfig() (parser.Parser, error) {
	m.Logger.Info("building the config")

	// load base config
	cfg, err := parser.New(options.Path(viper.GetString("haproxy.config.base")))
	if err != nil {
		m.Logger.Fatalw("failed to load haproxy base config", "error", err)
		return nil, err
	}

	return cfg, nil
}
