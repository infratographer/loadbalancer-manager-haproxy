package pkg

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/spf13/viper"
)

var dataPlaneClientTimeout = 2 * time.Second

// DataPlaneClient is the http client for Data Plane API
type DataPlaneClient struct {
	client  *http.Client
	baseURL string
}

// NewDataPlaneClient returns an http client for Data Plane API
func NewDataPlaneClient(url string) *DataPlaneClient {
	return &DataPlaneClient{
		client: &http.Client{
			Timeout: dataPlaneClientTimeout,
		},
		baseURL: url,
	}
}

// apiIsReady returns true when a 200 is returned for a GET request to the Data Plane API
func (c *DataPlaneClient) apiIsReady(ctx context.Context) bool {
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

// PostConfig pushes a new haproxy config in plain text using basic auth
func (c *DataPlaneClient) PostConfig(ctx context.Context, config string) error {
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
