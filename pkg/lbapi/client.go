package lbapi

import (
	"context"
	"net/http"

	"github.com/shurcooL/graphql"
	"go.infratographer.com/x/gidx"
)

// GQLClient is an interface for a graphql client
type GQLClient interface {
	Query(ctx context.Context, q interface{}, variables map[string]interface{}) error
}

// Client creates a new lb api client against a specific endpoint
type Client struct {
	client GQLClient
}

// NewClient creates a new lb api client
func NewClient(url string) *Client {
	return &Client{
		client: graphql.NewClient(url, &http.Client{}),
	}
}

// GetLoadBalancer returns a load balancer by id
func (c *Client) GetLoadBalancer(ctx context.Context, id string) (*GetLoadBalancer, error) {
	_, err := gidx.Parse(id)
	if err != nil {
		return nil, err
	}

	vars := map[string]interface{}{
		"id": id,
	}

	var lb GetLoadBalancer
	if err := c.client.Query(ctx, &lb, vars); err != nil {
		return nil, err
	}

	return &lb, nil
}
