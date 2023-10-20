package manager

import (
	"context"
	"fmt"
	"time"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/haproxytech/config-parser/v4/params"
	"github.com/haproxytech/config-parser/v4/types"

	lbapi "go.infratographer.com/load-balancer-api/pkg/client"

	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.uber.org/zap"
)

type lbAPI interface {
	GetLoadBalancer(ctx context.Context, id string) (*lbapi.LoadBalancer, error)
}

type dataPlaneAPI interface {
	PostConfig(ctx context.Context, config string) error
	CheckConfig(ctx context.Context, config string) error
	APIIsReady(ctx context.Context) bool
	WaitForDataPlaneReady(ctx context.Context, retries int, sleep time.Duration) error
}

type eventSubscriber interface {
	Listen() error
	Subscribe(topic string) error
}

// Manager contains configuration and client connections
type Manager struct {
	Context                       context.Context
	Logger                        *zap.SugaredLogger
	Subscriber                    eventSubscriber
	DataPlaneClient               dataPlaneAPI
	DataPlaneConnectRetries       int
	DataPlaneConnectRetryInterval time.Duration
	LBClient                      lbAPI
	ManagedLBID                   gidx.PrefixedID
	BaseCfgPath                   string

	// currentConfig for unit testing
	currentConfig string
}

// Run subscribes to a NATS subject and updates the haproxy config via dataplaneapi
func (m *Manager) Run() error {
	m.Logger.Info("Starting manager")

	if m.DataPlaneClient == nil {
		m.Logger.Fatal("dataplane api is not initialized")
	}

	if m.LBClient == nil {
		m.Logger.Fatal("loadbalancer api client is not initialized")
	}

	if m.Subscriber == nil {
		m.Logger.Fatal("pubsub subscriber client is not initialized")
	}

	// wait until the Data Plane API is running
	if err := m.DataPlaneClient.WaitForDataPlaneReady(m.Context, m.DataPlaneConnectRetries, m.DataPlaneConnectRetryInterval); err != nil {
		m.Logger.Fatal("unable to reach dataplaneapi. is it running?")
	}

	select {
	case <-m.Context.Done():
		return nil
	default:
		// use desired config on start
		if err := m.updateConfigToLatest(); err != nil {
			m.Logger.Fatalw("failed to initialize the config", zap.Error(err))
		}

		// listen for event messages on subject(s)
		if err := m.Subscriber.Listen(); err != nil {
			return err
		}
	}

	return nil
}

// loadbalancerTargeted returns true if this ChangeMessage is targeted to the
// loadbalancerID the manager is configured to act on
func (m Manager) loadbalancerTargeted(msg events.ChangeMessage) bool {
	m.Logger.Debugw("change msg received",
		"event-type", msg.EventType,
		"subjectID", msg.SubjectID,
		"additonalSubjects", msg.AdditionalSubjectIDs)

	if msg.SubjectID == m.ManagedLBID {
		return true
	} else {
		for _, subject := range msg.AdditionalSubjectIDs {
			if subject == m.ManagedLBID {
				return true
			}
		}
	}

	return false
}

// ProcessMsg message handler
func (m *Manager) ProcessMsg(msg events.Message[events.ChangeMessage]) error {
	changeMsg := msg.Message()

	mlogger := m.Logger.With(
		"event.message.id", msg.ID(),
		"event.message.topic", msg.Topic(),
		"event.message.source", msg.Source(),
		zap.String("loadbalancerID", m.ManagedLBID.String()),
		zap.String("event-type", changeMsg.EventType),
		zap.String("subjectID", changeMsg.SubjectID.String()),
		"additionalSubjects", changeMsg.AdditionalSubjectIDs)

	switch events.ChangeType(changeMsg.EventType) {
	case events.CreateChangeType:
		fallthrough
	case events.DeleteChangeType:
		fallthrough
	case events.UpdateChangeType:
		// drop msg, if not targeted for this lb
		if !m.loadbalancerTargeted(changeMsg) {
			return nil
		}

		mlogger.Infow("msg received")

		if err := m.updateConfigToLatest(); err != nil {
			mlogger.Errorw("failed to update haproxy config")
			return err
		}
	default:
		m.Logger.Debugw("ignoring msg, not a create/update/delete event",
			zap.String("event-type", changeMsg.EventType),
			zap.String("messageID", msg.ID()))
	}

	return nil
}

// updateConfigToLatest update the haproxy cfg to either baseline or one requested from lbapi with optional lbID param
func (m *Manager) updateConfigToLatest() error {
	m.Logger.Infow("updating haproxy config", zap.String("loadbalancerID", m.ManagedLBID.String()))

	if m.ManagedLBID == "" {
		return errLoadBalancerIDParamInvalid
	}

	// load base config
	cfg, err := parser.New(options.Path(m.BaseCfgPath), options.NoNamedDefaultsFrom)
	if err != nil {
		m.Logger.Fatalw("failed to load haproxy base config", zap.Error(err))
	}

	// get desired state from lbapi
	lb, err := m.LBClient.GetLoadBalancer(m.Context, m.ManagedLBID.String())
	if err != nil {
		return err
	}

	// merge response
	cfg, err = mergeConfig(cfg, lb)
	if err != nil {
		return err
	}

	// check dataplaneapi to see if a valid config
	if err := m.DataPlaneClient.CheckConfig(m.Context, cfg.String()); err != nil {
		return err
	}

	// post dataplaneapi
	if err := m.DataPlaneClient.PostConfig(m.Context, cfg.String()); err != nil {
		return err
	}

	m.Logger.Infow("config successfully updated", zap.String("loadbalancerID", m.ManagedLBID.String()))
	m.currentConfig = cfg.String() // for testing

	return nil
}

// mergeConfig takes the response from lb api, merges with the base haproxy config and returns it
func mergeConfig(cfg parser.Parser, lb *lbapi.LoadBalancer) (parser.Parser, error) {
	for _, p := range lb.Ports.Edges {
		// create port
		if err := cfg.SectionsCreate(parser.Frontends, p.Node.ID); err != nil {
			return nil, newLabelError(p.Node.ID, errFrontendSectionLabelFailure, err)
		}

		if err := cfg.Insert(parser.Frontends, p.Node.ID, "bind", types.Bind{
			// TODO AddressFamily?
			Path: fmt.Sprintf("%s@:%d", "ipv4", p.Node.Number)}); err != nil {
			return nil, newAttrError(errFrontendBindFailure, err)
		}

		// map frontend to backend
		if err := cfg.Set(parser.Frontends, p.Node.ID, "use_backend", types.UseBackend{Name: p.Node.ID}); err != nil {
			return nil, newAttrError(errUseBackendFailure, err)
		}

		// create backend
		if err := cfg.SectionsCreate(parser.Backends, p.Node.ID); err != nil {
			return nil, newLabelError(p.Node.ID, errBackendSectionLabelFailure, err)
		}

		for _, pool := range p.Node.Pools {
			for _, origin := range pool.Origins.Edges {
				var srvOptions []string

				srvAddr := fmt.Sprintf("%s:%d", origin.Node.Target, origin.Node.PortNumber)

				srvOptions = append(srvOptions, fmt.Sprintf("check port %d", origin.Node.PortNumber))
				srvOptions = append(srvOptions, fmt.Sprintf("weight %d", origin.Node.Weight))

				if !origin.Node.Active {
					srvOptions = append(srvOptions, "disabled")
				}

				srvParams := params.ParseServerOptions(srvOptions)
				if len(srvParams) != len(srvOptions) {
					return nil, newLabelError(p.Node.ID, errBackendServerFailure, errServerOptionParseFailure)
				}

				srvr := types.Server{
					Name:    origin.Node.ID,
					Address: srvAddr,
					Params:  srvParams,
				}

				if err := cfg.Set(parser.Backends, p.Node.ID, "server", srvr); err != nil {
					return nil, newLabelError(p.Node.ID, errBackendServerFailure, err)
				}
			}
		}
	}

	return cfg, nil
}
