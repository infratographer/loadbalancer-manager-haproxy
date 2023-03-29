package mock

import (
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

// HTTPClient is the mock http client
type HTTPClient struct {
	DoFunc func(req *retryablehttp.Request) (*http.Response, error)
}

func (c *HTTPClient) Do(req *retryablehttp.Request) (*http.Response, error) {
	return c.DoFunc(req)
}
