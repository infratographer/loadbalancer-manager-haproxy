package lbapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/lbapi/mock"
)

func newLBAPIMock(respJSON string, respCode int) *mock.HTTPClient {
	mockCli := &mock.HTTPClient{}
	mockCli.DoFunc = func(*retryablehttp.Request) (*http.Response, error) {
		json := respJSON

		r := io.NopCloser(strings.NewReader(json))
		return &http.Response{
			StatusCode: respCode,
			Body:       r,
		}, nil
	}

	return mockCli
}

func TestGetLoadBalancer(t *testing.T) {
	t.Run("GET v1/loadbalancers/:id", func(t *testing.T) {
		t.Parallel()
		respJSON := `{
					"id": "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f",
					"ports": [
						{
							"address_family": "ipv4",
							"id": "16dd23d7-d3ab-42c8-a645-3169f2659a0b",
							"name": "ssh-service",
							"port": 22,
							"pools": [
								"49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49"
							]
						}
					]
				}`
		cli := Client{
			baseURL: "test.url",
			client:  newLBAPIMock(respJSON, http.StatusOK),
		}

		lb, err := cli.GetLoadBalancer(context.Background(), "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f")
		require.Nil(t, err)

		assert.NotNil(t, lb)
		assert.Equal(t, "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f", lb.ID)
		assert.Len(t, lb.Ports, 1)
		assert.Equal(t, "ipv4", lb.Ports[0].AddressFamily)
		assert.Equal(t, "16dd23d7-d3ab-42c8-a645-3169f2659a0b", lb.Ports[0].ID)
		assert.Equal(t, "ssh-service", lb.Ports[0].Name)
		assert.Equal(t, int64(22), lb.Ports[0].Port)
		assert.Len(t, lb.Ports[0].Pools, 1)
		assert.Equal(t, "49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49", lb.Ports[0].Pools[0])
	})

	negativeTests := []struct {
		name            string
		respJSON        string
		respCode        int
		expectedFailure error
	}{
		{"GET v1/loadbalancers/:id - 401", "", http.StatusUnauthorized, ErrLBHTTPUnauthorized},
		{"GET v1/loadbalancers/:id - 500", "", http.StatusInternalServerError, ErrLBHTTPError},
		{"GET v1/loadbalancers/:id - other error", "", http.StatusBadRequest, ErrLBHTTPError},
	}

	for _, tt := range negativeTests {
		// go vet
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cli := Client{
				baseURL: "test.url",
				client:  newLBAPIMock(tt.respJSON, tt.respCode),
			}

			lb, err := cli.GetLoadBalancer(context.Background(), "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f")
			require.NotNil(t, err)
			assert.Nil(t, lb)
			assert.ErrorIs(t, err, tt.expectedFailure)
		})
	}
}

func TestGetPool(t *testing.T) {
	t.Run("GET v1/loadbalancers/pools/:id", func(t *testing.T) {
		t.Parallel()
		respJSON := `{
                    "id": "49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49",
                    "name": "ssh-service-a",
                    "origins": [
                        {
                            "id": "c0a80101-0000-0000-0000-000000000001",
                            "name": "svr1-2222",
                            "origin_target": "1.2.3.4",
                            "origin_disabled": false,
                            "port": 2222
                        }
                    ]
                }`
		cli := Client{
			baseURL: "test.url",
			client:  newLBAPIMock(respJSON, http.StatusOK),
		}

		pool, err := cli.GetPool(context.Background(), "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f")
		require.Nil(t, err)

		assert.NotNil(t, pool)
		assert.Equal(t, "49faa4a3-8d0b-4a7a-8bb9-7ed1b5995e49", pool.ID)
		assert.Equal(t, "ssh-service-a", pool.Name)
		require.Len(t, pool.Origins, 1)
		assert.Equal(t, "c0a80101-0000-0000-0000-000000000001", pool.Origins[0].ID)
		assert.Equal(t, "svr1-2222", pool.Origins[0].Name)
		assert.Equal(t, "1.2.3.4", pool.Origins[0].IPAddress)
		assert.Equal(t, false, pool.Origins[0].Disabled)
		assert.Equal(t, int64(2222), pool.Origins[0].Port)
	})

	negativeTests := []struct {
		name            string
		respJSON        string
		respCode        int
		expectedFailure error
	}{
		{"GET v1/loadbalancers/pools/:id - 401", "", http.StatusUnauthorized, ErrLBHTTPUnauthorized},
		{"GET v1/loadbalancers/pools/:id - 500", "", http.StatusInternalServerError, ErrLBHTTPError},
		{"GET v1/loadbalancers/pools/:id - other error", "", http.StatusBadRequest, ErrLBHTTPError},
	}

	for _, tt := range negativeTests {
		// go vet
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cli := Client{
				baseURL: "test.url",
				client:  newLBAPIMock(tt.respJSON, tt.respCode),
			}

			lb, err := cli.GetLoadBalancer(context.Background(), "58622a8d-54a2-4b0c-8b5f-8de7dff29f6f")
			require.NotNil(t, err)
			assert.Nil(t, lb)
			assert.ErrorIs(t, err, tt.expectedFailure)
		})
	}
}
