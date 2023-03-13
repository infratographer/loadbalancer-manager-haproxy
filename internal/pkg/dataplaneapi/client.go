// Package dataplaneapi contains the Data Plane API client and helper functions
package dataplaneapi

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	dataPlaneAPIRetryLimit = 10
	dataPlaneAPIRetrySleep = 1 * time.Second
	dataPlaneClientTimeout = 2 * time.Second
)

// Client is the http client for Data Plane API
type Client struct {
	client  *http.Client
	baseURL string
	logger  *zap.SugaredLogger
}

// Option is a functional configuration option
type Option func(c *Client)

// NewClient returns an http client for Data Plane API
func NewClient(opts ...Option) *Client {
	client := Client{
		client: &http.Client{
			Timeout: dataPlaneClientTimeout,
		},
		logger: zap.NewNop().Sugar(),
	}

	for _, opt := range opts {
		opt(&client)
	}

	return &client
}

// WithBaseURL sets the base URL for the Data Plane API
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// PostConfig pushes a new haproxy config in plain text using basic auth
func (c *Client) PostConfig(ctx context.Context, config string) error {
	url := c.baseURL + "/services/haproxy/configuration/raw?skip_version=true"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(config))
	if err != nil {
		return err
	}

	req.SetBasicAuth(viper.GetString("dataplane.user.name"), viper.GetString("dataplane.user.pwd"))
	req.Header.Add("Content-Type", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusUnauthorized:
		return ErrDataPlaneHTTPUnauthorized
	default:
		return ErrDataPlaneHTTPError
	}
}

// ApiIsReady returns true when a 200 is returned for a GET request to the Data Plane API
func (c *Client) apiIsReady(ctx context.Context) bool {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	req.SetBasicAuth(viper.GetString("dataplane.user.name"), viper.GetString("dataplane.user.pwd"))

	resp, err := c.client.Do(req)
	if err != nil {
		// likely connection timeout
		return false
	}

	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// WaitForDataPlaneReady will check if the Data Plane API is returning 200 and retry if not
func (c *Client) WaitForDataPlaneReady(ctx context.Context) error {
	for i := 0; i < dataPlaneAPIRetryLimit; i++ {
		if c.apiIsReady(ctx) {
			c.logger.Info("dataplaneapi is ready")
			return nil
		}

		c.logger.Info("waiting for dataplaneapi to become ready")
		time.Sleep(dataPlaneAPIRetrySleep)
	}

	return ErrDataPlaneNotReady
}
