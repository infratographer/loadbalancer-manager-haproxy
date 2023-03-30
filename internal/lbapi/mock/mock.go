package mock

import (
	"net/http"
)

// HTTPClient is the mock http client
type HTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.DoFunc(req)
}
