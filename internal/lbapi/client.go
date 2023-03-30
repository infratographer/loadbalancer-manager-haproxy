// lbapi TODO: will move to https://github.com/infratographer/load-balancer-api
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

const (
	apiVersion = "v1"
)

// HTTPClient interface
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	client  HTTPClient
	baseURL string
}

func NewClient(url string, opts ...func(*Client)) *Client {
	// default retryable http client
	retryCli := retryablehttp.NewClient()
	retryCli.RetryMax = 3
	retryCli.HTTPClient.Timeout = time.Second * 5
	retryCli.Logger = nil

	c := &Client{
		baseURL: url,
		client:  retryCli.HTTPClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithHTTPClient inject your specific http client
func WithHTTPClient(httpClient *http.Client) func(*Client) {
	return func(c *Client) {
		c.client = httpClient
	}
}

// GetLoadBalancer returns a load balancer by id
func (c Client) GetLoadBalancer(ctx context.Context, id string) (*LoadBalancerResponse, error) {
	lb := &LoadBalancerResponse{}
	url := fmt.Sprintf("%s/%s/loadbalancers/%s", c.baseURL, apiVersion, id)

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req.Request)
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

// GetPool returns a load balancer pool  by id
func (c Client) GetPool(ctx context.Context, id string) (*PoolResponse, error) {
	pool := &PoolResponse{}
	url := fmt.Sprintf("%s/%s/loadbalancers/pools/%s", c.baseURL, apiVersion, id)

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req.Request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(pool); err != nil {
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

	return pool, nil
}
