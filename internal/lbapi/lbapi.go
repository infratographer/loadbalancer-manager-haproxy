package lbapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type Frontend struct {
	ID   string `json:"uuid"`
	Port int64  `json:"port"`
}

type Assignment struct {
	ID       string   `json:"uuid"`
	Frontend Frontend `json:"frontend"`
	Pools    []Pool   `json:"pools"`
}

type Origin struct {
	ID        string `json:"uuid"`
	Name      string `json:"name"`
	IPAddress string `json:"address"`
	Enabled   bool   `json:"enabled"`
	Port      string `json:"port"`
}

type Pool struct {
	ID      string   `json:"uuid"`
	Name    string   `json:"name"`
	Origins []Origin `json:"origins"`
}

type LB struct {
	ID          string       `json:"lb_uuid"`
	IPAddress   string       `json:"lb_ip_address"`
	Slug        string       `json:"slug"`
	Assignments []Assignment `json:"assignments"`
}

type Client struct {
	client  *retryablehttp.Client
	baseURL string
}

func NewClient(url string, opts ...func(*Client)) *Client {
	retryCli := retryablehttp.NewClient()
	retryCli.RetryMax = 3
	retryCli.HTTPClient.Timeout = time.Second * 5

	c := &Client{
		baseURL: url,
		client:  retryCli,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithRetries(r int) func(*Client) {
	return func(c *Client) {
		c.client.RetryMax = r
	}
}

func WithTimeout(timeout time.Duration) func(*Client) {
	return func(c *Client) {
		c.client.HTTPClient.Timeout = timeout
	}
}

func (c Client) GetLoadBalancer(ctx context.Context, id string) (*LB, error) {
	url := fmt.Sprintf("%s/loadbalancers/%s", c.baseURL, id)
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// TODO: auth
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// TODO @rizzza - check status

	lb := &LB{}
	if err := json.NewDecoder(resp.Body).Decode(lb); err != nil {
		return nil, err
	}

	return lb, nil
}
