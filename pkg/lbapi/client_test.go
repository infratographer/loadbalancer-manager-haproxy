package lbapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/loadbalancer-manager-haproxy/pkg/lbapi/internal/mock"
)

func newGQLClientMock() *mock.GQLClient {
	mockCli := &mock.GQLClient{}
	mockCli.DoQuery = func(ctx context.Context, q interface{}, variables map[string]interface{}) error {
		lb, ok := q.(*GetLoadBalancer)
		if ok {
			lb.LoadBalancer.ID = "loadbal-test"
			lb.LoadBalancer.Name = "test"
		}

		return nil
	}

	return mockCli
}

func TestGetLoadBalancer(t *testing.T) {
	cli := Client{
		gqlCli: newGQLClientMock(),
	}

	lb, err := cli.GetLoadBalancer(context.Background(), "badprefix-test")
	require.Error(t, err)
	require.Nil(t, lb)
	assert.ErrorContains(t, err, "invalid id")

	lb, err = cli.GetLoadBalancer(context.Background(), "loadbal-test")
	require.NoError(t, err)
	require.NotNil(t, lb)

	assert.Equal(t, lb.LoadBalancer.ID, "loadbal-test")
	assert.Equal(t, lb.LoadBalancer.Name, "test")
}
