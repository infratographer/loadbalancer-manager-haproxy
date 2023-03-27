package lbapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

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

func (c Client) GetLoadBalancer(ctx context.Context, id string) (*LoadBalancer, error) {
	lb := &LoadBalancer{}
	url := fmt.Sprintf("%s/loadbalancers/%s?verbose=true", c.baseURL, id)

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(lb); err != nil {
			return nil, fmt.Errorf("failed to decode load balancer: %v", err)
		}
	case http.StatusNotFound:
		return nil, ErrLBHTTPNotfound
	case http.StatusUnauthorized:
		return nil, ErrLBHTTPUnauthorized
	case http.StatusInternalServerError:
		return nil, ErrLBHTTPError
	default:
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read resp body")
		}
		return nil, fmt.Errorf("%s: %w", fmt.Sprintf("StatusCode (%d) - %s ", resp.StatusCode, string(b)), ErrLBHTTPError)
	}

	return lb, nil
}
