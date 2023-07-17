package lbapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/shurcooL/graphql"
	"go.infratographer.com/x/gidx"
)

// GQLClient is an interface for a graphql client
type GQLClient interface {
	Query(ctx context.Context, q interface{}, variables map[string]interface{}) error
}

// Client creates a new lb api client against a specific endpoint
type Client struct {
	gqlCli     GQLClient
	httpClient *http.Client
}

// ClientOption is a function that modifies a client
type ClientOption func(*Client)

// NewClient creates a new lb api client
func NewClient(url string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.gqlCli = graphql.NewClient(url, c.httpClient)

	return c
}

// WithHTTPClient functional option to set the http client
func WithHTTPClient(cli *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = cli
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
	if err := c.gqlCli.Query(ctx, &lb, vars); err != nil {
		return nil, translateGQLErr(err)
	}

	return &lb, nil
}

func translateGQLErr(err error) error {
	if strings.Contains(err.Error(), "load_balancer not found") {
		return ErrLBNotfound
	} else if strings.Contains(err.Error(), "invalid or expired jwt") {
		return ErrUnauthorized
	}

	return err
}
