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

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/manager/mock"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pubsub"
	"go.infratographer.com/loadbalancer-manager-haproxy/pkg/lbapi"
)

const (
	testDataBaseDir = "testdata"
	testBaseCfgPath = "../../.devcontainer/config/haproxy.cfg"
)

func TestMergeConfig(t *testing.T) {
	MergeConfigTests := []struct {
		name                string
		testInput           lbapi.GetLoadBalancer
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

			cfg, err := parser.New(options.Path("../../.devcontainer/config/haproxy.cfg"), options.NoNamedDefaultsFrom)
			require.Nil(t, err)

			newCfg, err := mergeConfig(cfg, &tt.testInput)
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
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.GetLoadBalancer, error) {
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
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.GetLoadBalancer, error) {
				return &lbapi.GetLoadBalancer{
					LoadBalancer: lbapi.LoadBalancer{
						ID:    "loadbal-test",
						Ports: lbapi.Ports{},
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
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.GetLoadBalancer, error) {
				return &lbapi.GetLoadBalancer{
					LoadBalancer: lbapi.LoadBalancer{
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
																Active:     true,
															},
														},
														{
															Node: lbapi.OriginNode{
																ID:         "loadogn-test2",
																Name:       "svr1-222",
																Target:     "1.2.3.4",
																PortNumber: 222,
																Active:     true,
															},
														},
														{
															Node: lbapi.OriginNode{
																ID:         "loadogn-test3",
																Name:       "svr2",
																Target:     "4.3.2.1",
																PortNumber: 2222,
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

	mgr := Manager{
		Logger:      logger,
		ManagedLBID: gidx.PrefixedID("loadbal-managedbythisprocess"),
		Context:     context.Background(),
	}

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
			t.Parallel()

			msg := CreateTestMessage(t, &mgr, tt.pubsubMsg)
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
		t.Parallel()

		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoCheckConfig: func(ctx context.Context, config string) error {
				return nil
			},
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
		}

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.GetLoadBalancer, error) {
				return &lbapi.GetLoadBalancer{
					LoadBalancer: lbapi.LoadBalancer{
						ID: "loadbal-managedbythisprocess",
					},
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

		msg := CreateTestMessage(t, mgr, events.ChangeMessage{
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

	t.Run("events integration", func(t *testing.T) {
		t.Parallel()

		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoCheckConfig: func(ctx context.Context, config string) error {
				return nil
			},
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
		}

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.GetLoadBalancer, error) {
				return &lbapi.GetLoadBalancer{
					LoadBalancer: lbapi.LoadBalancer{
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
																Active:     true,
															},
														},
														{
															Node: lbapi.OriginNode{
																ID:         "loadogn-test2",
																Name:       "svr1-222",
																Target:     "1.2.3.4",
																PortNumber: 222,
																Active:     true,
															},
														},
														{
															Node: lbapi.OriginNode{
																ID:         "loadogn-test3",
																Name:       "svr2",
																Target:     "4.3.2.1",
																PortNumber: 2222,
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
		}

		// setup timeout context to break free from pubsub Listen()
		ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(1*time.Second))
		defer cancel()

		mgr.Context = ctx

		_ = CreateTestMessage(t, mgr, events.ChangeMessage{
			SubjectID: gidx.PrefixedID("loadbal-managedbythisprocess"),
			EventType: string(events.CreateChangeType),
		})

		err := mgr.Subscriber.Listen()
		require.Nil(t, err)

		// check currentConfig (testing helper variable)
		assert.NotEmpty(t, mgr.currentConfig)

		expCfg, err := os.ReadFile(fmt.Sprintf("%s/%s", testDataBaseDir, "lb-ex-1-exp.cfg"))
		require.Nil(t, err)

		assert.Equal(t, strings.TrimSpace(string(expCfg)), strings.TrimSpace(mgr.currentConfig))
	})
}

func CreateTestMessage(t *testing.T, mgr *Manager, changeMsg events.ChangeMessage) events.Message[events.ChangeMessage] {
	// testnats server connection
	natsSrv, err := eventtools.NewNatsServer()
	require.NoError(t, err)

	eventHandler, err := events.NewNATSConnection(natsSrv.Config.NATS)
	require.NoError(t, err)

	// subscribe
	subscriber := pubsub.NewSubscriber(mgr.Context, eventHandler, pubsub.WithMsgHandler(mgr.ProcessMsg))
	require.NotNil(t, subscriber)

	mgr.Subscriber = subscriber

	err = mgr.Subscriber.Subscribe("create.loadbalancer")
	require.NoError(t, err)

	// publish
	eventsConn, err := events.NewConnection(natsSrv.Config)
	require.NoError(t, err)

	testMsg, err := eventsConn.PublishChange(
		mgr.Context,
		"loadbalancer",
		changeMsg)
	require.NoError(t, err)

	return testMsg
}

var mergeTestData1 = lbapi.GetLoadBalancer{
	LoadBalancer: lbapi.LoadBalancer{
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
												Active:     true,
											},
										},
										{
											Node: lbapi.OriginNode{
												ID:         "loadogn-test2",
												Name:       "svr1-222",
												Target:     "1.2.3.4",
												PortNumber: 222,
												Active:     true,
											},
										},
										{
											Node: lbapi.OriginNode{
												ID:         "loadogn-test3",
												Name:       "svr2",
												Target:     "4.3.2.1",
												PortNumber: 2222,
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
	},
}

var mergeTestData2 = lbapi.GetLoadBalancer{
	LoadBalancer: lbapi.LoadBalancer{
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
												Active:     true,
											},
										},
										{
											Node: lbapi.OriginNode{
												ID:         "loadogn-test2",
												Name:       "svr1-222",
												Target:     "1.2.3.4",
												PortNumber: 222,
												Active:     true,
											},
										},
										{
											Node: lbapi.OriginNode{
												ID:         "loadogn-test3",
												Name:       "svr2",
												Target:     "4.3.2.1",
												PortNumber: 2222,
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
	},
}

var mergeTestData3 = lbapi.GetLoadBalancer{
	LoadBalancer: lbapi.LoadBalancer{
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
	},
}
