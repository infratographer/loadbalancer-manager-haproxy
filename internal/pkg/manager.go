package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/haproxytech/config-parser/v4/types"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/dataplaneapi"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/lbapi"

	"go.infratographer.com/x/pubsubx"
	"go.infratographer.com/x/urnx"
	"go.uber.org/zap"
	"gocloud.dev/pubsub"
	"gocloud.dev/pubsub/natspubsub"
)

var (
	dataPlaneAPIRetryLimit = 10
	dataPlaneAPIRetrySleep = 1 * time.Second
)

type lbAPI interface {
	GetLoadBalancer(ctx context.Context, id string) (*lbapi.LoadBalancer, error)
	GetPool(ctx context.Context, id string) (*lbapi.Pool, error)
}

// ManagerConfig contains configuration and client connections
type ManagerConfig struct {
	Context         context.Context
	Logger          *zap.SugaredLogger
	NatsConn        *nats.Conn
	DataPlaneClient dataPlaneAPI
	LBClient        lbAPI
}

// Run subscribes to a NATS subject and updates the haproxy config via dataplaneapi
func (m *ManagerConfig) Run() error {
	// wait until the Data Plane API is running
	if err := m.waitForDataPlaneReady(dataPlaneAPIRetryLimit, dataPlaneAPIRetrySleep); err != nil {
		m.Logger.Fatal("unable to reach dataplaneapi. is it running?")
	}

	// use desired config on start
	if err := m.updateConfigToLatest(); err != nil {
		m.Logger.Errorw("failed to initialize the config", zap.Error(err))
	}

	// subscribe to nats queue -> update config to latest on msg receive
	subject := viper.GetString("nats.subject")

	subscription, err := natspubsub.OpenSubscription(m.NatsConn, subject, nil)
	if err != nil {
		// TODO - update
		m.Logger.Errorw("failed to subscribe to queue ", zap.String("subject", subject))
		return err
	}

	m.Logger.Infow("subscribed to NATS subject ", zap.String("subject", subject))

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

		_ = m.processMsg(msg)

		// TODO - @rizzza - √√ on this ack with Tyler. We ack everything? nack is fatal with this driver.
		msg.Ack()
	}
}

// processMsg message handler
func (m ManagerConfig) processMsg(msg *pubsub.Message) error {
	pubsubMsg := pubsubx.Message{}
	if err := json.Unmarshal(msg.Body, &pubsubMsg); err != nil {
		m.Logger.Errorw("failed to process data in msg", zap.Error(err))
		return err
	}

	urn, err := urnx.Parse(pubsubMsg.SubjectURN)
	if err != nil {
		m.Logger.Errorw("failed to parse pubsub msg urn", zap.String("subjectURN", pubsubMsg.SubjectURN), zap.Error(err))
		return err
	}

	lbID := urn.ResourceID.String()
	if err = m.updateConfigToLatest(lbID); err != nil {
		m.Logger.Errorw("failed to update haproxy config", zap.String("loadbalancer.id", lbID), zap.Error(err))
		return err
	}

	return nil
}

// updateConfigToLatest update the haproxy cfg to either baseline or one requested from lbapi with optional lbID param
func (m ManagerConfig) updateConfigToLatest(lbID ...string) error {
	m.Logger.Info("updating the config")

	// load base config
	cfg, err := parser.New(options.Path(viper.GetString("haproxy.config.base")), options.NoNamedDefaultsFrom)
	if err != nil {
		m.Logger.Fatalw("failed to load haproxy base config", "error", err)
	}

	lbAPIConfigured := len(viper.GetString("loadbalancerapi.url")) > 0
	if !lbAPIConfigured {
		m.Logger.Warn("loadbalancerapi.url is not configured: defaulting to base haproxy config")
	}

	if len(lbID) == 1 && lbAPIConfigured {
		// requested a lb id, query lbapi
		// get desired state
		lb, err := m.LBClient.GetLoadBalancer(m.Context, lbID[0])
		if err != nil {
			return err
		}

		// query each pool and copy the origins into our lb datastructure
		for _, port := range lb.Ports {
			for _, poolID := range port.Pools {
				// query poolID
				p, err := m.LBClient.GetPool(m.Context, poolID)
				if err != nil {
					return err
				}

				data := lbapi.Pool{
					ID:      poolID,
					Name:    p.Name,
					Origins: p.Origins,
				}
				port.PoolData = append(port.PoolData, data)
			}
		}

		// merge response
		cfg, err = mergeConfig(cfg, lb)
		if err != nil {
			return err
		}
	}

	// post dataplaneapi
	if err = m.DataPlaneClient.PostConfig(m.Context, cfg.String()); err != nil {
		return err
	}

	m.Logger.Info("config successfully updated")

	return nil
}

func (m ManagerConfig) waitForDataPlaneReady(retries int, sleep time.Duration) error {
	for i := 0; i < retries; i++ {
		if m.DataPlaneClient.ApiIsReady(m.Context) {
			m.Logger.Info("dataplaneapi is ready")
			return nil
		}

		m.Logger.Info("waiting for dataplaneapi to become ready")
		time.Sleep(sleep)
	}

	return dataplaneapi.ErrDataPlaneNotReady
}

// mergeConfig takes the response from lb api, merges with the base haproxy config and returns it
func mergeConfig(cfg parser.Parser, lb *lbapi.LoadBalancer) (parser.Parser, error) {
	for _, p := range lb.Ports {
		// create port
		if err := cfg.SectionsCreate(parser.Frontends, p.Name); err != nil {
			return nil, fmt.Errorf("failed to create frontend section with label %q: %w", p.Name, err)
		}

		if err := cfg.Insert(parser.Frontends, p.Name, "bind", types.Bind{
			Path: fmt.Sprintf("%s@:%d", p.AddressFamily, p.Port)}); err != nil {
			return nil, fmt.Errorf("failed to create frontend attr bind: %w", err)
		}

		// map frontend to backend
		if err := cfg.Set(parser.Frontends, p.Name, "use_backend", types.UseBackend{Name: p.Name}); err != nil {
			return nil, fmt.Errorf("failed to create frontend attr use_backend: %w", err)
		}

		// create backend
		if err := cfg.SectionsCreate(parser.Backends, p.Name); err != nil {
			return nil, fmt.Errorf("failed to create section backend with label %q': %w", p.Name, err)
		}

		for _, pool := range p.PoolData {
			for _, origin := range pool.Origins {
				srvAddr := fmt.Sprintf("%s:%d check port %d", origin.IPAddress, origin.Port, origin.Port)

				if origin.Disabled {
					srvAddr += " disabled"
				}

				srvr := types.Server{
					Name:    origin.ID,
					Address: srvAddr,
				}

				if err := cfg.Set(parser.Backends, p.Name, "server", srvr); err != nil {
					return nil, fmt.Errorf("failed to add backend %q attr server: %w", p.Name, err)
				}
			}
		}
	}

	return cfg, nil
}
