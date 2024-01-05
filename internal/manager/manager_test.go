package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/testing/eventtools"

	lbapi "go.infratographer.com/load-balancer-api/pkg/client"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/manager/mock"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pubsub"
)

const (
	testDataBaseDir = "testdata"
	testBaseCfgPath = "../../.devcontainer/config/haproxy.cfg"
)

func TestMergeConfig(t *testing.T) {
	l, err := zap.NewDevelopmentConfig().Build()
	logger := l.Sugar()

	require.Nil(t, err)

	MergeConfigTests := []struct {
		name                string
		testInput           lbapi.LoadBalancer
		expectedCfgFilename string
	}{
		{"ssh service one pool", mergeTestData1, "lb-ex-1-exp.cfg"},
		{"ssh service two pools", mergeTestData2, "lb-ex-2-exp.cfg"},
		{"http and https", mergeTestData3, "lb-ex-3-exp.cfg"},
	}

	for _, tt := range MergeConfigTests {
		// go vet
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := Manager{
				Logger: logger,
			}

			cfg, err := parser.New(options.Path("../../.devcontainer/config/haproxy.cfg"), options.NoNamedDefaultsFrom)
			require.Nil(t, err)

			newCfg, err := mgr.mergeConfig(cfg, &tt.testInput)
			assert.Nil(t, err)

			t.Log("Generated config ===> ", newCfg.String())

			expCfg, err := os.ReadFile(fmt.Sprintf("%s/%s", testDataBaseDir, tt.expectedCfgFilename))
			require.Nil(t, err)

			assert.Equal(t, strings.TrimSpace(string(expCfg)), strings.TrimSpace(newCfg.String()))
		})
	}
}

func TestUpdateConfigToLatest(t *testing.T) {
	l, err := zap.NewDevelopmentConfig().Build()
	logger := l.Sugar()

	require.Nil(t, err)

	t.Run("failure to query for loadbalancer", func(t *testing.T) {
		t.Parallel()

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancer, error) {
				return nil, fmt.Errorf("failure") // nolint:goerr113
			},
		}

		mgr := Manager{
			Logger:      logger,
			LBClient:    mockLBAPI,
			BaseCfgPath: testBaseCfgPath,
			ManagedLBID: gidx.PrefixedID("loadbal-testing"),
		}

		err := mgr.updateConfigToLatest()
		assert.NotNil(t, err)
	})

	t.Run("fails to update invalid config", func(t *testing.T) {
		t.Parallel()

		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
			DoCheckConfig: func(ctx context.Context, config string) error {
				return errors.New("bad config") // nolint:goerr113
			},
		}

		mgr := Manager{
			Logger:          logger,
			DataPlaneClient: mockDataplaneAPI,
			BaseCfgPath:     testBaseCfgPath,
		}

		// initial config
		err := mgr.updateConfigToLatest()
		require.Error(t, err)
	})

	t.Run("errors when manager loadbalancerID is empty", func(t *testing.T) {
		mgr := Manager{
			Logger:      logger,
			BaseCfgPath: testBaseCfgPath,
		}

		err := mgr.updateConfigToLatest()
		require.ErrorIs(t, err, errLoadBalancerIDParamInvalid)
	})

	t.Run("successfully sets initial base config", func(t *testing.T) {
		t.Parallel()

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancer, error) {
				return &lbapi.LoadBalancer{
					ID:    "loadbal-test",
					Ports: lbapi.Ports{},
				}, nil
			},
		}

		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
			DoCheckConfig: func(ctx context.Context, config string) error {
				return nil
			},
		}

		mgr := Manager{
			Logger:          logger,
			DataPlaneClient: mockDataplaneAPI,
			LBClient:        mockLBAPI,
			BaseCfgPath:     testBaseCfgPath,
			ManagedLBID:     gidx.PrefixedID("loadbal-test"),
		}

		err := mgr.updateConfigToLatest()
		require.Nil(t, err)

		contents, err := os.ReadFile(testBaseCfgPath)
		require.Nil(t, err)

		// remove that 'unnamed_defaults_1' thing the haproxy parser library puts in the default section,
		// even though the library is configured to not include default section labels
		mgr.currentConfig = strings.ReplaceAll(mgr.currentConfig, " unnamed_defaults_1", "")

		assert.Equal(t, strings.TrimSpace(string(contents)), strings.TrimSpace(mgr.currentConfig))
	})

	t.Run("successfully queries lb api and merges changes with base config", func(t *testing.T) {
		t.Parallel()

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancer, error) {
				return &lbapi.LoadBalancer{
					ID: "loadbal-test",
					Ports: lbapi.Ports{
						Edges: []lbapi.PortEdges{
							{
								Node: lbapi.PortNode{
									ID:     "loadprt-test",
									Name:   "ssh-service",
									Number: 22,
									Pools: []lbapi.Pool{
										{
											ID:       "loadpol-test",
											Name:     "ssh-service-a",
											Protocol: "tcp",
											Origins: lbapi.Origins{
												Edges: []lbapi.OriginEdges{
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test1",
															Name:       "svr1-2222",
															Target:     "1.2.3.4",
															PortNumber: 2222,
															Weight:     20,
															Active:     true,
														},
													},
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test2",
															Name:       "svr1-222",
															Target:     "1.2.3.4",
															PortNumber: 222,
															Weight:     30,
															Active:     true,
														},
													},
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test3",
															Name:       "svr2",
															Target:     "4.3.2.1",
															PortNumber: 2222,
															Weight:     50,
															Active:     false,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}

		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
			DoCheckConfig: func(ctx context.Context, config string) error {
				return nil
			},
		}

		mgr := Manager{
			Logger:          logger,
			LBClient:        mockLBAPI,
			DataPlaneClient: mockDataplaneAPI,
			BaseCfgPath:     testBaseCfgPath,
			ManagedLBID:     gidx.PrefixedID("loadbal-test"),
		}

		err := mgr.updateConfigToLatest()
		require.Nil(t, err)

		expCfg, err := os.ReadFile(fmt.Sprintf("%s/%s", testDataBaseDir, "lb-ex-1-exp.cfg"))
		require.Nil(t, err)

		assert.Equal(t, strings.TrimSpace(string(expCfg)), strings.TrimSpace(mgr.currentConfig))
	})

	t.Run("successfully queries lb api and merges changes with base config while excluding private ips in backend config", func(t *testing.T) {
		t.Parallel()

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancer, error) {
				return &lbapi.LoadBalancer{
					ID: "loadbal-test",
					Ports: lbapi.Ports{
						Edges: []lbapi.PortEdges{
							{
								Node: lbapi.PortNode{
									ID:     "loadprt-test",
									Name:   "ssh-service",
									Number: 22,
									Pools: []lbapi.Pool{
										{
											ID:       "loadpol-test",
											Name:     "ssh-service-a",
											Protocol: "tcp",
											Origins: lbapi.Origins{
												Edges: []lbapi.OriginEdges{
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test1",
															Name:       "svr1-2222",
															Target:     "1.2.3.4",
															PortNumber: 2222,
															Weight:     20,
															Active:     true,
														},
													},
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test2",
															Name:       "svr1-222",
															Target:     "1.2.3.4",
															PortNumber: 222,
															Weight:     30,
															Active:     true,
														},
													},
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test3",
															Name:       "svr2",
															Target:     "4.3.2.1",
															PortNumber: 2222,
															Weight:     50,
															Active:     false,
														},
													},
													{ // private ip will be skipped
														Node: lbapi.OriginNode{
															ID:         "loadogn-test4",
															Name:       "svr2",
															Target:     "10.0.0.0",
															PortNumber: 2222,
															Weight:     50,
															Active:     false,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}

		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
			DoCheckConfig: func(ctx context.Context, config string) error {
				return nil
			},
		}

		mgr := Manager{
			Logger:          logger,
			LBClient:        mockLBAPI,
			DataPlaneClient: mockDataplaneAPI,
			BaseCfgPath:     testBaseCfgPath,
			ManagedLBID:     gidx.PrefixedID("loadbal-test"),
		}

		err := mgr.updateConfigToLatest()
		require.Error(t, err)
	})
}

func TestLoadBalancerTargeted(t *testing.T) {
	l, _ := zap.NewDevelopmentConfig().Build()
	logger := l.Sugar()

	testcases := []struct {
		name             string
		pubsubMsg        events.ChangeMessage
		msgTargetedForLB bool
	}{
		{
			name: "subjectID targeted for loadbalancer",
			pubsubMsg: events.ChangeMessage{
				SubjectID:            gidx.PrefixedID("loadbal-testing"),
				AdditionalSubjectIDs: []gidx.PrefixedID{"loadpol-testing"},
			},
			msgTargetedForLB: true,
		},
		{
			name: "AdditionalSubjectID is targeted for loadbalancer",
			pubsubMsg: events.ChangeMessage{
				SubjectID:            gidx.PrefixedID("loadprt-testing"),
				AdditionalSubjectIDs: []gidx.PrefixedID{"loadbal-testing"},
			},
			msgTargetedForLB: true,
		},
		{
			name: "msg is not targeted for loadbalancer",
			pubsubMsg: events.ChangeMessage{
				SubjectID:            gidx.PrefixedID("loadprt-nottargeted"),
				AdditionalSubjectIDs: []gidx.PrefixedID{"loadbal-nottargeted"},
			},
			msgTargetedForLB: false,
		},
	}

	mgr := Manager{
		ManagedLBID: gidx.PrefixedID("loadbal-testing"),
		Logger:      logger,
	}

	for _, tt := range testcases {
		// go vet
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			targeted := mgr.loadbalancerTargeted(tt.pubsubMsg)
			assert.Equal(t, tt.msgTargetedForLB, targeted)
		})
	}
}

func TestProcessMsg(t *testing.T) {
	l, err := zap.NewDevelopmentConfig().Build()
	logger := l.Sugar()

	require.Nil(t, err)

	// testnats server connection
	natsSrv, err := eventtools.NewNatsServer()
	require.NoError(t, err)

	eventsConn, err := events.NewNATSConnection(natsSrv.Config.NATS)
	require.NoError(t, err)

	defer func() {
		natsSrv.Close()

		_ = eventsConn.Shutdown(context.Background())
	}()

	mgr := Manager{
		Logger:      logger,
		ManagedLBID: gidx.PrefixedID("loadbal-managedbythisprocess"),
		Context:     context.Background(),
	}

	// subscribe
	subscriber := pubsub.NewSubscriber(context.Background(), eventsConn, pubsub.WithMsgHandler(mgr.ProcessMsg))
	require.NotNil(t, subscriber)

	err = subscriber.Subscribe("*.loadbalancer")
	require.NoError(t, err)

	mgr.Subscriber = subscriber

	ProcessMsgTests := []struct {
		name      string
		pubsubMsg events.ChangeMessage
		errMsg    string
	}{
		{
			name:      "ignores messages with subject prefix not supported",
			pubsubMsg: events.ChangeMessage{SubjectID: "invalid-", EventType: string(events.CreateChangeType)},
		},
		{
			name:      "ignores messages not targeted for this lb",
			pubsubMsg: events.ChangeMessage{SubjectID: gidx.PrefixedID("loadbal-test"), EventType: string(events.CreateChangeType)},
		},
	}

	for _, tt := range ProcessMsgTests {
		// go vet
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			msg := PublishTestMessage(t, context.Background(), eventsConn, tt.pubsubMsg)
			err := mgr.ProcessMsg(msg)

			if tt.errMsg != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)
				return
			}

			assert.NoError(t, err)
		})
	}

	t.Run("successfully process create msg", func(t *testing.T) {
		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoCheckConfig: func(ctx context.Context, config string) error {
				return nil
			},
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
		}

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancer, error) {
				return &lbapi.LoadBalancer{
					ID: "loadbal-managedbythisprocess",
				}, nil
			},
		}

		mgr := &Manager{
			Context:         context.Background(),
			Logger:          logger,
			DataPlaneClient: mockDataplaneAPI,
			LBClient:        mockLBAPI,
			ManagedLBID:     gidx.PrefixedID("loadbal-managedbythisprocess"),
		}

		msg := PublishTestMessage(t, mgr.Context, eventsConn, events.ChangeMessage{
			SubjectID: gidx.PrefixedID("loadbal-managedbythisprocess"),
			EventType: string(events.CreateChangeType),
		})

		err = mgr.ProcessMsg(msg)
		require.Nil(t, err)
	})
}

func TestEventsIntegration(t *testing.T) {
	l, _ := zap.NewDevelopmentConfig().Build()
	logger := l.Sugar()

	// testnats server connection
	natsSrv, err := eventtools.NewNatsServer()
	require.NoError(t, err)

	eventsConn, err := events.NewNATSConnection(natsSrv.Config.NATS)
	require.NoError(t, err)

	defer func() {
		natsSrv.Close()

		_ = eventsConn.Shutdown(context.Background())
	}()

	t.Run("events integration", func(t *testing.T) {
		ctx := context.Background()

		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoCheckConfig: func(ctx context.Context, config string) error {
				return nil
			},
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
		}

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancer, error) {
				return &lbapi.LoadBalancer{
					ID: "loadbal-managedbythisprocess",
					Ports: lbapi.Ports{
						Edges: []lbapi.PortEdges{
							{
								Node: lbapi.PortNode{
									ID:     "loadprt-test",
									Name:   "ssh-service",
									Number: 22,
									Pools: []lbapi.Pool{
										{
											ID:       "loadpol-test",
											Name:     "ssh-service-a",
											Protocol: "tcp",
											Origins: lbapi.Origins{
												Edges: []lbapi.OriginEdges{
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test1",
															Name:       "svr1-2222",
															Target:     "1.2.3.4",
															PortNumber: 2222,
															Weight:     20,
															Active:     true,
														},
													},
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test2",
															Name:       "svr1-222",
															Target:     "1.2.3.4",
															PortNumber: 222,
															Weight:     30,
															Active:     true,
														},
													},
													{
														Node: lbapi.OriginNode{
															ID:         "loadogn-test3",
															Name:       "svr2",
															Target:     "4.3.2.1",
															PortNumber: 2222,
															Weight:     50,
															Active:     false,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}

		mgr := &Manager{
			BaseCfgPath:     "../../.devcontainer/config/haproxy.cfg",
			Logger:          logger,
			DataPlaneClient: mockDataplaneAPI,
			LBClient:        mockLBAPI,
			ManagedLBID:     gidx.PrefixedID("loadbal-managedbythisprocess"),
			Context:         ctx,
		}

		// subscribe
		subscriber := pubsub.NewSubscriber(ctx, eventsConn, pubsub.WithMsgHandler(mgr.ProcessMsg))
		require.NotNil(t, subscriber)

		err = subscriber.Subscribe(">")
		require.NoError(t, err)

		mgr.Subscriber = subscriber

		go func() {
			err := mgr.Subscriber.Listen()
			require.Nil(t, err)
		}()

		_ = PublishTestMessage(t, ctx, eventsConn, events.ChangeMessage{
			SubjectID: gidx.PrefixedID("loadbal-managedbythisprocess"),
			EventType: string(events.CreateChangeType),
		})

		// wait for msg to be processed by manager
		time.Sleep(1 * time.Second)

		// check currentConfig (testing helper variable)
		assert.NotEmpty(t, mgr.currentConfig)

		expCfg, err := os.ReadFile(fmt.Sprintf("%s/%s", testDataBaseDir, "lb-ex-1-exp.cfg"))
		require.Nil(t, err)

		assert.Equal(t, strings.TrimSpace(string(expCfg)), strings.TrimSpace(mgr.currentConfig))
	})
}

func PublishTestMessage(t *testing.T, ctx context.Context, eventsConn events.Connection, changeMsg events.ChangeMessage) events.Message[events.ChangeMessage] {
	// publish
	testMsg, err := eventsConn.PublishChange(
		ctx,
		"create.loadbalancer",
		changeMsg)
	require.NoError(t, err)

	return testMsg
}

var mergeTestData1 = lbapi.LoadBalancer{
	ID:   "loadbal-test",
	Name: "test",
	Ports: lbapi.Ports{
		Edges: []lbapi.PortEdges{
			{
				Node: lbapi.PortNode{
					// TODO - @rizzza - AddressFamily?
					ID:     "loadprt-test",
					Name:   "ssh-service",
					Number: 22,
					Pools: []lbapi.Pool{
						{
							ID:       "loadpol-test",
							Name:     "ssh-service-a",
							Protocol: "tcp",
							Origins: lbapi.Origins{
								Edges: []lbapi.OriginEdges{
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test1",
											Name:       "svr1-2222",
											Target:     "1.2.3.4",
											PortNumber: 2222,
											Weight:     20,
											Active:     true,
										},
									},
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test2",
											Name:       "svr1-222",
											Target:     "1.2.3.4",
											PortNumber: 222,
											Weight:     30,
											Active:     true,
										},
									},
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test3",
											Name:       "svr2",
											Target:     "4.3.2.1",
											PortNumber: 2222,
											Weight:     50,
											Active:     false,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

var mergeTestData2 = lbapi.LoadBalancer{
	ID:   "loadbal-test",
	Name: "test",
	Ports: lbapi.Ports{
		Edges: []lbapi.PortEdges{
			{
				Node: lbapi.PortNode{
					// TODO - @rizzza - AddressFamily?
					ID:     "loadprt-test",
					Name:   "ssh-service-a",
					Number: 22,
					Pools: []lbapi.Pool{
						{
							ID:       "loadpol-test",
							Name:     "ssh-service-a",
							Protocol: "tcp",
							Origins: lbapi.Origins{
								Edges: []lbapi.OriginEdges{
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test1",
											Name:       "svr1-2222",
											Target:     "1.2.3.4",
											PortNumber: 2222,
											Weight:     20,
											Active:     true,
										},
									},
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test2",
											Name:       "svr1-222",
											Target:     "1.2.3.4",
											PortNumber: 222,
											Weight:     30,
											Active:     true,
										},
									},
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test3",
											Name:       "svr2",
											Target:     "4.3.2.1",
											PortNumber: 2222,
											Weight:     50,
											Active:     false,
										},
									},
								},
							},
						},
						{
							ID:       "loadpol-test2",
							Name:     "ssh-service-b",
							Protocol: "tcp",
							Origins: lbapi.Origins{
								Edges: []lbapi.OriginEdges{
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test4",
											Name:       "svr1-2222",
											Target:     "7.8.9.0",
											PortNumber: 2222,
											Weight:     100,
											Active:     true,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

var mergeTestData3 = lbapi.LoadBalancer{
	ID:   "loadbal-test",
	Name: "http/https",
	Ports: lbapi.Ports{
		Edges: []lbapi.PortEdges{
			{
				Node: lbapi.PortNode{
					// TODO - @rizzza - AddressFamily?
					ID:     "loadprt-testhttp",
					Name:   "http",
					Number: 80,
					Pools: []lbapi.Pool{
						{
							ID:       "loadpol-test",
							Name:     "ssh-service-a",
							Protocol: "tcp",
							Origins: lbapi.Origins{
								Edges: []lbapi.OriginEdges{
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test1",
											Name:       "svr1",
											Target:     "3.1.4.1",
											PortNumber: 80,
											Weight:     1,
											Active:     true,
										},
									},
								},
							},
						},
					},
				},
			},
			{
				Node: lbapi.PortNode{
					// TODO - @rizzza - AddressFamily?
					ID:     "loadprt-testhttps",
					Name:   "https",
					Number: 443,
					Pools: []lbapi.Pool{
						{
							ID:       "loadpol-test",
							Name:     "ssh-service-a",
							Protocol: "tcp",
							Origins: lbapi.Origins{
								Edges: []lbapi.OriginEdges{
									{
										Node: lbapi.OriginNode{
											ID:         "loadogn-test2",
											Name:       "svr1",
											Target:     "3.1.4.1",
											PortNumber: 443,
											Weight:     90,
											Active:     true,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}
