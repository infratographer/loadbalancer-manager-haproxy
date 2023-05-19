package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/manager/mock"
	"go.infratographer.com/loadbalancer-manager-haproxy/pkg/lbapi"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/pubsubx"
	"go.uber.org/zap"
)

const (
	testDataBaseDir = "testdata"
	testBaseCfgPath = "../../.devcontainer/config/haproxy.cfg"
)

func TestMergeConfig(t *testing.T) {
	MergeConfigTests := []struct {
		name                string
		testInput           loadBalancer
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

	t.Run("errors on failure to query for loadbalancers/:id", func(t *testing.T) {
		t.Parallel()

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancerResponse, error) {
				return nil, fmt.Errorf("failure") // nolint:goerr113
			},
		}

		mgr := Manager{
			Logger:      logger,
			LBClient:    mockLBAPI,
			BaseCfgPath: testBaseCfgPath,
		}

		err := mgr.updateConfigToLatest("58622a8d-54a2-4b0c-8b5f-8de7dff29f6f")
		assert.NotNil(t, err)
	})

	t.Run("errors on failure to query for loadbalancers/pools/:id", func(t *testing.T) {
		t.Parallel()

		mockLBAPI := &mock.LBAPIClient{
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancerResponse, error) {
				return &lbapi.LoadBalancerResponse{
					LoadBalancer: lbapi.LoadBalancer{
						Ports: []lbapi.Port{
							{
								Name:          "ssh-service",
								AddressFamily: "ipv4",
								Port:          22,
								ID:            "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
								Pools: []string{
									"49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49",
								},
							},
						},
					},
				}, nil
			},
			DoGetPool: func(ctx context.Context, id string) (*lbapi.PoolResponse, error) {
				return nil, fmt.Errorf("failure") // nolint:goerr113
			},
		}

		mgr := Manager{
			Logger:      logger,
			LBClient:    mockLBAPI,
			BaseCfgPath: testBaseCfgPath,
		}

		err := mgr.updateConfigToLatest("58622a8d-54a2-4b0c-8b5f-8de7dff29f6f")
		assert.NotNil(t, err)
	})

	t.Run("successfully sets initial base config", func(t *testing.T) {
		t.Parallel()

		mockDataplaneAPI := &mock.DataplaneAPIClient{
			DoPostConfig: func(ctx context.Context, config string) error {
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
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancerResponse, error) {
				return &lbapi.LoadBalancerResponse{
					LoadBalancer: lbapi.LoadBalancer{
						ID: "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f",
						Ports: []lbapi.Port{
							{
								Name:          "ssh-service",
								AddressFamily: "ipv4",
								Port:          22,
								ID:            "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
								Pools: []string{
									"49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49",
								},
							},
						},
					},
				}, nil
			},
			DoGetPool: func(ctx context.Context, id string) (*lbapi.PoolResponse, error) {
				return &lbapi.PoolResponse{
					Pool: lbapi.Pool{
						ID:   "49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49",
						Name: "ssh-service-a",
						Origins: []lbapi.Origin{
							{
								ID:        "c0a80101-0000-0000-0000-000000000001",
								Name:      "svr1-2222",
								IPAddress: "1.2.3.4",
								Disabled:  false,
								Port:      2222,
							},
							{
								ID:        "c0a80101-0000-0000-0000-000000000002",
								Name:      "svr1-222",
								IPAddress: "1.2.3.4",
								Disabled:  false,
								Port:      222,
							},
							{
								ID:        "c0a80101-0000-0000-0000-000000000003",
								Name:      "svr2",
								IPAddress: "4.3.2.1",
								Disabled:  true,
								Port:      2222,
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
		}

		mgr := Manager{
			Logger:          logger,
			LBClient:        mockLBAPI,
			DataPlaneClient: mockDataplaneAPI,
			BaseCfgPath:     testBaseCfgPath,
		}

		err := mgr.updateConfigToLatest("58622a8d-54a2-4b0c-8b5f-8de7dff29f6f")
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
			DoGetLoadBalancer: func(ctx context.Context, id string) (*lbapi.LoadBalancerResponse, error) {
				return &lbapi.LoadBalancerResponse{
					LoadBalancer: lbapi.LoadBalancer{
						ID:    "loadbal-managedbythisprocess",
						Ports: []lbapi.Port{},
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

var mergeTestData1 = loadBalancer{
	ID: "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f",
	Ports: []port{
		{
			Name:          "ssh-service",
			AddressFamily: "ipv4",
			Port:          22,
			ID:            "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
			Pools: []pool{
				{
					ID:   "49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49",
					Name: "ssh-service-a",
					Origins: []origin{
						{
							ID:        "c0a80101-0000-0000-0000-000000000001",
							Name:      "svr1-2222",
							IPAddress: "1.2.3.4",
							Disabled:  false,
							Port:      2222,
						},
						{
							ID:        "c0a80101-0000-0000-0000-000000000002",
							Name:      "svr1-222",
							IPAddress: "1.2.3.4",
							Disabled:  false,
							Port:      222,
						},
						{
							ID:        "c0a80101-0000-0000-0000-000000000003",
							Name:      "svr2",
							IPAddress: "4.3.2.1",
							Disabled:  true,
							Port:      2222,
						},
					},
				},
			},
		},
	},
}

var mergeTestData2 = loadBalancer{
	ID: "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f",
	Ports: []port{
		{
			Name:          "ssh-service",
			AddressFamily: "ipv4",
			Port:          22,
			ID:            "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
			Pools: []pool{
				{
					ID:   "49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49",
					Name: "ssh-service-a",
					Origins: []origin{
						{
							ID:        "c0a80101-0000-0000-0000-000000000001",
							Name:      "svr1-2222",
							IPAddress: "1.2.3.4",
							Disabled:  false,
							Port:      2222,
						},
						{
							ID:        "c0a80101-0000-0000-0000-000000000002",
							Name:      "svr1-222",
							IPAddress: "1.2.3.4",
							Disabled:  false,
							Port:      222,
						},
						{
							ID:        "c0a80101-0000-0000-0000-000000000003",
							Name:      "svr2",
							IPAddress: "4.3.2.1",
							Disabled:  true,
							Port:      2222,
						},
					},
				},
				{
					ID:   "c9bd57ac-6d88-4786-849e-0b228c17d645",
					Name: "ssh-service-b",
					Origins: []origin{
						{
							ID:        "b1982331-0000-0000-0000-000000000001",
							Name:      "svr1-2222",
							IPAddress: "7.8.9.0",
							Disabled:  false,
							Port:      2222,
						},
					},
				},
			},
		},
	},
}

var mergeTestData3 = loadBalancer{
	ID: "a522bc95-2a74-4005-919d-6ae0a5be056d",
	Ports: []port{
		{
			Name:          "http",
			AddressFamily: "ipv4",
			Port:          80,
			ID:            "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
			Pools: []pool{
				{
					ID:   "49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49",
					Name: "ssh-service-a",
					Origins: []origin{
						{
							ID:        "c0a80101-0000-0000-0000-000000000001",
							Name:      "svr1",
							IPAddress: "3.1.4.1",
							Disabled:  false,
							Port:      80,
						},
					},
				},
			},
		},
		{
			Name:          "https",
			AddressFamily: "ipv4",
			Port:          443,
			ID:            "8ca812cc-9c3d-4fed-95be-40a773f7d876",
			Pools: []pool{
				{
					ID:   "d94ad98b-b074-4794-896f-d71ae3b7b0ac",
					Name: "ssh-service-a",
					Origins: []origin{
						{
							ID:        "676a1536-0a17-4676-9296-ee957e5871c1",
							Name:      "svr1",
							IPAddress: "3.1.4.1",
							Disabled:  false,
							Port:      443,
						},
					},
				},
			},
		},
	},
}
