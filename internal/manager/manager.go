package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/haproxytech/config-parser/v4/types"
	"github.com/nats-io/nats.go"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/dataplaneapi"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pubsub"
	"go.infratographer.com/loadbalancer-manager-haproxy/pkg/lbapi"

	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/pubsubx"
	"go.uber.org/zap"
)

var (
	dataPlaneAPIRetryLimit = 10
	dataPlaneAPIRetrySleep = 1 * time.Second
)

type lbAPI interface {
	GetLoadBalancer(ctx context.Context, id string) (*lbapi.LoadBalancerResponse, error)
	GetPool(ctx context.Context, id string) (*lbapi.PoolResponse, error)
}

type dataPlaneAPI interface {
	PostConfig(ctx context.Context, config string) error
	APIIsReady(ctx context.Context) bool
}

// Manager contains configuration and client connections
type Manager struct {
	Context         context.Context
	Logger          *zap.SugaredLogger
	NatsClient      *pubsub.NatsClient
	DataPlaneClient dataPlaneAPI
	LBClient        lbAPI
	ManagedLBID     string
	BaseCfgPath     string

	// primarily for testing
	currentConfig string
}

// Run subscribes to a NATS subject and updates the haproxy config via dataplaneapi
func (m *Manager) Run() error {
	m.Logger.Info("Starting manager")

	// wait until the Data Plane API is running
	if err := m.waitForDataPlaneReady(dataPlaneAPIRetryLimit, dataPlaneAPIRetrySleep); err != nil {
		m.Logger.Fatal("unable to reach dataplaneapi. is it running?")
	}

	// use desired config on start
	if err := m.updateConfigToLatest(); err != nil {
		m.Logger.Errorw("failed to initialize the config", zap.Error(err))
	}

	// listen for nats messages on subject(s)
	if err := m.NatsClient.Listen(); err != nil {
		return err
	}

	return nil
}

const (
	subjectPrefixLoadBalancer     = "loadbal"
	subjectPrefixLoadBalancerPort = "loadprt"

	eventTypeCreate = "create"
	eventTypeUpdate = "update"
)

// supportedPrefix returns true if the subject prefix is supported by this manager
func supportedPrefix(prefix string) bool {
	switch prefix {
	case subjectPrefixLoadBalancer:
		fallthrough
	case subjectPrefixLoadBalancerPort:
		return true
	default:
		return false
	}
}

// getTargetLoadBalancerID returns the loadbalancer id from the message,
// whether it is the SubjectID, or one of the AdditionalSubjectIds
func getTargetLoadBalancerID(msg *pubsubx.ChangeMessage) (gidx.PrefixedID, error) {
	var lbID gidx.PrefixedID

	if msg.SubjectID.Prefix() == subjectPrefixLoadBalancer {
		lbID = msg.SubjectID
	} else {
		for _, subject := range msg.AdditionalSubjectIDs {
			if subject.Prefix() == subjectPrefixLoadBalancer {
				lbID = subject
			}
		}
	}

	if lbID.String() == "" {
		return "", errLoadbalancerIDNotFound
	}

	// check if a valid gidx
	lbID, err := gidx.Parse(lbID.String())
	if err != nil {
		return "", err
	}

	return lbID, nil
}

// ProcessMsg message handler
func (m Manager) ProcessMsg(msg *nats.Msg) error {
	pubsubMsg := pubsubx.ChangeMessage{}
	if err := json.Unmarshal(msg.Data, &pubsubMsg); err != nil {
		m.Logger.Errorw("failed to process data in msg", zap.Error(err))
		return err
	}

	subjectPrefix := pubsubMsg.SubjectID.Prefix()
	if !supportedPrefix(subjectPrefix) {
		m.Logger.Debugw("ignoring msg, not a supported prefix", zap.String("subject-prefix", subjectPrefix))
		return nil
	}

	switch pubsubMsg.EventType {
	case eventTypeCreate:
		fallthrough
	case eventTypeUpdate:
		targetLoadBalancerID, err := getTargetLoadBalancerID(&pubsubMsg)
		if err != nil {
			m.Logger.Errorw("failed to get target loadbalancer id", zap.Error(err))
			return err
		}

		// drop msg, if not targeted for this lb
		if targetLoadBalancerID.String() != m.ManagedLBID {
			m.Logger.Debugw("ignoring msg, not targeted for this lb", zap.String("loadbalancer-id", m.ManagedLBID))
			return nil
		}

		if err := m.updateConfigToLatest(targetLoadBalancerID.String()); err != nil {
			m.Logger.Errorw("failed to update haproxy config",
				zap.String("loadbalancer.id", targetLoadBalancerID.String()),
				zap.Error(err))

			return err
		}

		if err := msg.Ack(); err != nil {
			m.Logger.Errorw("failed to ack msg", zap.Error(err), zap.String("subjectID", pubsubMsg.SubjectID.String()))
			return err
		}
	default:
		m.Logger.Debugw("ignoring msg, not a create or update event", zap.String("event-type", pubsubMsg.EventType))
	}

	return nil
}

// updateConfigToLatest update the haproxy cfg to either baseline or one requested from lbapi with optional lbID param
func (m *Manager) updateConfigToLatest(lbID ...string) error {
	if len(lbID) > 1 {
		return errLoadBalancerIDInvalid
	}

	m.Logger.Info("updating the config")

	// load base config
	cfg, err := parser.New(options.Path(m.BaseCfgPath), options.NoNamedDefaultsFrom)
	if err != nil {
		m.Logger.Fatalw("failed to load haproxy base config", "error", err)
	}

	if len(lbID) == 1 {
		m.Logger.Warn("graph client not implemented yet to update haproxy cfg")
		return nil

		// requested a lb id, query lbapi
		// get desired state
		// lbResp, err := m.LBClient.GetLoadBalancer(m.Context, lbID[0])
		// if err != nil {
		// 	return err
		// }

		// lb := loadBalancer{
		// 	ID: lbResp.LoadBalancer.ID,
		// }

		// // translate responses, populate data structure
		// for i, p := range lbResp.LoadBalancer.Ports {
		// 	lb.Ports = append(lb.Ports, port{
		// 		AddressFamily: p.AddressFamily,
		// 		ID:            p.ID,
		// 		Name:          p.Name,
		// 		Port:          p.Port,
		// 	})

		// 	for _, poolID := range p.Pools {
		// 		poolResp, err := m.LBClient.GetPool(m.Context, poolID)
		// 		if err != nil {
		// 			return err
		// 		}

		// 		data := pool{
		// 			ID:   poolID,
		// 			Name: poolResp.Pool.Name,
		// 		}

		// 		for _, o := range poolResp.Pool.Origins {
		// 			data.Origins = append(data.Origins, origin{
		// 				ID:        o.ID,
		// 				Name:      o.Name,
		// 				IPAddress: o.IPAddress,
		// 				Disabled:  o.Disabled,
		// 				Port:      o.Port,
		// 			})
		// 		}

		// 		lb.Ports[i].Pools = append(lb.Ports[i].Pools, data)
		// 	}
		// }

		// // merge response
		// cfg, err = mergeConfig(cfg, &lb)
		// if err != nil {
		// 	return err
		// }
	}

	// post dataplaneapi
	if err = m.DataPlaneClient.PostConfig(m.Context, cfg.String()); err != nil {
		return err
	}

	m.Logger.Info("config successfully updated")
	m.currentConfig = cfg.String() // primarily for testing

	return nil
}

func (m Manager) waitForDataPlaneReady(retries int, sleep time.Duration) error {
	for i := 0; i < retries; i++ {
		if m.DataPlaneClient.APIIsReady(m.Context) {
			m.Logger.Info("dataplaneapi is ready")
			return nil
		}

		m.Logger.Info("waiting for dataplaneapi to become ready")
		time.Sleep(sleep)
	}

	return dataplaneapi.ErrDataPlaneNotReady
}

// mergeConfig takes the response from lb api, merges with the base haproxy config and returns it
func mergeConfig(cfg parser.Parser, lb *loadBalancer) (parser.Parser, error) {
	for _, p := range lb.Ports {
		// create port
		if err := cfg.SectionsCreate(parser.Frontends, p.Name); err != nil {
			return nil, newLabelError(p.Name, errFrontendSectionLabelFailure, err)
		}

		if err := cfg.Insert(parser.Frontends, p.Name, "bind", types.Bind{
			Path: fmt.Sprintf("%s@:%d", p.AddressFamily, p.Port)}); err != nil {
			return nil, newAttrError(errFrontendBindFailure, err)
		}

		// map frontend to backend
		if err := cfg.Set(parser.Frontends, p.Name, "use_backend", types.UseBackend{Name: p.Name}); err != nil {
			return nil, newAttrError(errUseBackendFailure, err)
		}

		// create backend
		if err := cfg.SectionsCreate(parser.Backends, p.Name); err != nil {
			return nil, newLabelError(p.Name, errBackendSectionLabelFailure, err)
		}

		for _, pool := range p.Pools {
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
					return nil, newLabelError(p.Name, errBackendServerFailure, err)
				}
			}
		}
	}

	return cfg, nil
}
