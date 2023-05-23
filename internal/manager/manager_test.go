package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/pubsubx"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/manager/mock"
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
		}

		err := mgr.updateConfigToLatest("loadbal-test")
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

	t.Run("successfully sets initial base config", func(t *testing.T) {
		t.Parallel()

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
			BaseCfgPath:     testBaseCfgPath,
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
		}

		err := mgr.updateConfigToLatest("loadbal-test")
		require.Nil(t, err)

		expCfg, err := os.ReadFile(fmt.Sprintf("%s/%s", testDataBaseDir, "lb-ex-1-exp.cfg"))
		require.Nil(t, err)

		assert.Equal(t, strings.TrimSpace(string(expCfg)), strings.TrimSpace(mgr.currentConfig))
	})
}

func TestGetTargetLoadBalancerID(t *testing.T) {
	testcases := []struct {
		name          string
		pubsubMsg     pubsubx.ChangeMessage
		exptectedLBID gidx.PrefixedID
		errMsg        string
	}{
		{
			name:      "failure to parse invalid subjectID",
			pubsubMsg: pubsubx.ChangeMessage{SubjectID: "loadbal-"},
			errMsg:    "invalid id",
		},
		{
			name: "failure when loadbalancer id not found in the msg",
			pubsubMsg: pubsubx.ChangeMessage{SubjectID: "loadprt-test",
				AdditionalSubjectIDs: []gidx.PrefixedID{"loadpol-test"}},
			errMsg: "not found",
		},
		{
			name:          "get target loadbalancer gixd from SubjectID",
			exptectedLBID: "loadbal-test",
			pubsubMsg: pubsubx.ChangeMessage{SubjectID: "loadbal-test",
				AdditionalSubjectIDs: []gidx.PrefixedID{"loadpol-test"}},
		},
		{
			name:          "get target loadbalancer gixd from AdditionalSubjectIDs",
			exptectedLBID: "loadbal-test",
			pubsubMsg: pubsubx.ChangeMessage{SubjectID: "loadprt-test",
				AdditionalSubjectIDs: []gidx.PrefixedID{"loadbal-test"}},
		},
	}

	for _, tt := range testcases {
		// go vet
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lbID, err := getTargetLoadBalancerID(&tt.pubsubMsg)

			if tt.errMsg != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.exptectedLBID, lbID)
		})
	}
}

func TestProcessMsg(t *testing.T) {
	l, err := zap.NewDevelopmentConfig().Build()
	logger := l.Sugar()

	require.Nil(t, err)

	mgr := Manager{
		Logger:      logger,
		ManagedLBID: "loadbal-managedbythisprocess",
	}

	ProcessMsgTests := []struct {
		name      string
		pubsubMsg interface{}
		errMsg    string
	}{
		{
			name:      "failure to unmarshal msg",
			pubsubMsg: "not a valid msg",
			errMsg:    "cannot unmarshal",
		},
		{
			name:      "ignores messages with subject prefix not supported",
			pubsubMsg: pubsubx.ChangeMessage{SubjectID: "invalid-", EventType: eventTypeCreate},
		},
		{
			name:      "ignores messages not targeted for this lb",
			pubsubMsg: pubsubx.ChangeMessage{SubjectID: "loadbal-test", EventType: eventTypeCreate},
		},
	}

	for _, tt := range ProcessMsgTests {
		// go vet
		tt := tt

		data, _ := json.Marshal(tt.pubsubMsg)

		natsMsg := &nats.Msg{
			Subject: "test.subject",
			Data:    data,
		}

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := mgr.ProcessMsg(natsMsg)

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
			DoPostConfig: func(ctx context.Context, config string) error {
				return nil
			},
		}

		mockNatsClient := &mock.NatsClient{
			DoAck: func(msg *nats.Msg) error {
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

		mgr := Manager{
			Logger:          logger,
			DataPlaneClient: mockDataplaneAPI,
			NatsClient:      mockNatsClient,
			LBClient:        mockLBAPI,
			ManagedLBID:     "loadbal-managedbythisprocess",
		}

		data, _ := json.Marshal(pubsubx.ChangeMessage{
			SubjectID: "loadbal-managedbythisprocess",
			EventType: eventTypeCreate,
		})

		natsMsg := &nats.Msg{
			Subject: "test.subject",
			Data:    data,
		}

		err := mgr.ProcessMsg(natsMsg)
		require.Nil(t, err)
	})
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
