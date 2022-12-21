package pkg

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"gocloud.dev/pubsub/natspubsub"
)

type Manager struct {
	Context  context.Context
	Logger   *zap.SugaredLogger
	NatsConn *nats.Conn
}

func (m *Manager) Run(ctx context.Context) error {
	// use desired config on start
	if err := m.updateConfigToLatest(); err != nil {
		m.Logger.Error("failed to update the config", "error", err)
	}

	// subscribe to nats queue -> doTheThings on msg receive
	sub, err := natspubsub.OpenSubscription(
		m.NatsConn,
		"example.mysubject",
		nil)
	if err != nil {
		// TODO - update
		m.Logger.Error("failed to subscribe to queue")
		return err
	}
	defer sub.Shutdown(ctx)

	for {
		msg, err := sub.Receive(ctx)
		if err != nil {
			m.Logger.Error("failed receiving nats message")
			return err
		}

		m.Logger.Info("received nats message", "message", msg.Body)

		if err = m.updateConfigToLatest(); err != nil {
			m.Logger.Error("failed to update the config", "error", err)
		}

		msg.Ack()
	}
}

func (m *Manager) updateConfigToLatest() error {
	m.Logger.Info("updating the config")
	// load base config
	// get desired state
	// transform response
	// merge desired with base
	// post dataplaneapi
	return nil
}
