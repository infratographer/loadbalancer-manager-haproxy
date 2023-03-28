package pkg

import (
	"fmt"
	"os"
	"strings"
	"testing"

	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/options"
	"github.com/stretchr/testify/assert"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/lbapi"
)

const (
	testDataBaseDir = "testdata"
)

func TestMergeConfig(t *testing.T) {
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

			cfg, err := parser.New(options.Path("../../.devcontainer/config/haproxy.cfg"), options.NoNamedDefaultsFrom)
			assert.Nil(t, err)

			newCfg, err := mergeConfig(cfg, &tt.testInput)
			assert.Nil(t, err)

			t.Log("Generated config ===> ", newCfg.String())

			expCfg, err := os.ReadFile(fmt.Sprintf("%s/%s", testDataBaseDir, tt.expectedCfgFilename))
			assert.Nil(t, err)

			assert.Equal(t, strings.TrimSpace(string(expCfg)), strings.TrimSpace(newCfg.String()))
		})
	}
}

var mergeTestData1 = lbapi.LoadBalancer{
	ID: "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f",
	Ports: []lbapi.Port{
		{
			Name:          "ssh-service",
			AddressFamily: "ipv4",
			Port:          22,
			ID:            "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
			PoolData: []lbapi.Pool{
				{
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
			},
		},
	},
}

var mergeTestData2 = lbapi.LoadBalancer{
	ID: "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f",
	Ports: []lbapi.Port{
		{
			Name:          "ssh-service",
			AddressFamily: "ipv4",
			Port:          22,
			ID:            "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
			PoolData: []lbapi.Pool{
				{
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
				{
					ID:   "c9bd57ac-6d88-4786-849e-0b228c17d645",
					Name: "ssh-service-b",
					Origins: []lbapi.Origin{
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

var mergeTestData3 = lbapi.LoadBalancer{
	ID: "a522bc95-2a74-4005-919d-6ae0a5be056d",
	Ports: []lbapi.Port{
		{
			Name:          "http",
			AddressFamily: "ipv4",
			Port:          80,
			ID:            "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
			PoolData: []lbapi.Pool{
				{
					ID:   "49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49",
					Name: "ssh-service-a",
					Origins: []lbapi.Origin{
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
			PoolData: []lbapi.Pool{
				{
					ID:   "d94ad98b-b074-4794-896f-d71ae3b7b0ac",
					Name: "ssh-service-a",
					Origins: []lbapi.Origin{
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
