package lbapi

import (
	"context"
	"net/http"

	"github.com/shurcooL/graphql"
)

// Client creates a new lb api client against a specific endpoint
type Client struct {
	client *graphql.Client
}

// NewClient creates a new lb api client
func NewClient(url string) *Client {
	return &Client{
		client: graphql.NewClient(url, &http.Client{}),
	}
}

// GetLoadBalancer returns a load balancer by id
func (c *Client) GetLoadBalancer(ctx context.Context, id string) (*GetLoadBalancer, error) {
	vars := map[string]interface{}{
		"id": id,
	}

	var lb GetLoadBalancer
	if err := c.client.Query(ctx, &lb, vars); err != nil {
		return nil, err
	}

	return &lb, nil
}
