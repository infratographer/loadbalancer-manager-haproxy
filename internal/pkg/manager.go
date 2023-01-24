package pkg

import (
	"context"
	"errors"
	"time"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gocloud.dev/pubsub/natspubsub"
)

var (
	dataPlaneAPIRetryLimit = 10
	dataPlaneAPIRetrySleep = 1 * time.Second
)

// ManagerConfig contains configuration and client connections
type ManagerConfig struct {
	Context         context.Context
	Logger          *zap.SugaredLogger
	NatsConn        *nats.Conn
	DataPlaneClient *DataPlaneClient
}

// Run subscribes to a NATS subject and updates the haproxy config via dataplaneapi
func (m *ManagerConfig) Run() error {
	// wait until the Data Plane API is running
	if err := m.waitForDataPlaneReady(dataPlaneAPIRetryLimit, dataPlaneAPIRetrySleep); err != nil {
		m.Logger.Fatal("unable to reach dataplaneapi. is it running?")
	}

	// use desired config on start
	if err := m.updateConfigToLatest(); err != nil {
		m.Logger.Error("failed to update the config", "error", err)
	}

	// subscribe to nats queue -> update config to latest on msg receive
	subject := viper.GetString("nats.subject")

	subscription, err := natspubsub.OpenSubscription(m.NatsConn, subject, nil)
	if err != nil {
		// TODO - update
		m.Logger.Error("failed to subscribe to queue ", "subject: ", subject)
		return err
	}

	m.Logger.Info("subscribed to NATS subject ", "subject: ", subject)

	defer func() {
		_ = subscription.Shutdown(m.Context)
	}()

	for {
		msg, err := subscription.Receive(m.Context)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				m.Logger.Info("context canceled")
				return nil
			}

			m.Logger.Error("failed receiving nats message")

			return err
		}

		m.Logger.Info("received nats message ", "message: ", string(msg.Body))

		if err = m.updateConfigToLatest(); err != nil {
			m.Logger.Error("failed to update the config", "error", err)
		}

		msg.Ack()
	}
}

func (m *ManagerConfig) updateConfigToLatest() error {
	m.Logger.Info("updating the config")
	// load base config
	cfg, err := parser.New(options.Path(viper.GetString("haproxy.config.base")))
	if err != nil {
		m.Logger.Fatalw("failed to load haproxy base config", "error", err)
	}

	// get desired state
	// transform response
	// merge desired with base

	// post dataplaneapi
	if err = m.DataPlaneClient.PostConfig(m.Context, cfg.String()); err != nil {
		m.Logger.Error("failed to post new haproxy config", "error", err)
	}

	return err
}

func (m *ManagerConfig) waitForDataPlaneReady(retries int, sleep time.Duration) error {
	for i := 0; i < retries; i++ {
		if m.DataPlaneClient.apiIsReady(m.Context) {
			m.Logger.Info("dataplaneapi is ready")
			return nil
		}

		m.Logger.Info("waiting for dataplaneapi to become ready")
		time.Sleep(sleep)
	}

	return ErrDataPlaneNotReady
}
