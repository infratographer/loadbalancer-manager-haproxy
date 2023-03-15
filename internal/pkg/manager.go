package pkg

import (
	"context"
	"errors"
	"fmt"
	"time"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/haproxytech/config-parser/v4/types"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/lbapi"
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
	LBClient        *lbapi.Client
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
	cfg, err := parser.New(options.Path(viper.GetString("haproxy.config.base")), options.NoNamedDefaultsFrom)
	if err != nil {
		m.Logger.Fatalw("failed to load haproxy base config", "error", err)
	}

	// get desired state
	lb := lbapi.LB{}

	// merge response
	newCfg, err := mergeConfig(cfg, &lb)
	if err != nil {
		m.Logger.Error("failed to merge haproxy config", "error", err)
		return err
	}

	// post dataplaneapi
	if err = m.DataPlaneClient.PostConfig(m.Context, newCfg.String()); err != nil {
		m.Logger.Error("failed to post new haproxy config", "error", err)
		return err
	}

	return nil
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

// mergeConfig takes the response from lb api, modifies the base haproxy config then returns it
func mergeConfig(cfg parser.Parser, lb *lbapi.LB) (parser.Parser, error) {
	if len(lb.Assignments) <= 0 {
		return nil, fmt.Errorf("failed to recieve any assignments for load balancer %q", lb.ID)
	}

	for _, a := range lb.Assignments {
		// create frontend
		if err := cfg.SectionsCreate(parser.Frontends, a.Frontend.ID); err != nil {
			return nil, fmt.Errorf("failed to create frontend section with ID %q: %w", a.Frontend.ID, err)
		}

		if err := cfg.Insert(parser.Frontends, a.Frontend.ID, "bind", types.Bind{
			Path: fmt.Sprintf("ipv4@:%d", a.Frontend.Port)}); err != nil {
			return nil, fmt.Errorf("failed to create frontend attr bind: %w", err)
		}

		// map frontend to backend
		if err := cfg.Set(parser.Frontends, a.Frontend.ID, "use_backend", types.UseBackend{Name: a.ID}); err != nil {
			return nil, fmt.Errorf("failed to create frontend attr use_backend: %w", err)
		}

		// create backends
		if err := cfg.SectionsCreate(parser.Backends, a.ID); err != nil {
			return nil, fmt.Errorf("failed to create section backend with ID %q': %w", a.ID, err)
		}

		// TODO? check for no pools
		for _, pool := range a.Pools {
			for _, origin := range pool.Origins {
				srvAddr := fmt.Sprintf("%s:%s check port %s", origin.IPAddress, origin.Port, origin.Port)

				if !origin.Enabled {
					srvAddr += " disabled"
				}

				srvr := types.Server{
					Name:    origin.ID,
					Address: srvAddr,
				}

				if err := cfg.Set(parser.Backends, a.ID, "server", srvr); err != nil {
					return nil, fmt.Errorf("failed to add backend %q attr server: %w", a.ID, err)
				}
			}
		}
	}

	return cfg, nil
}
