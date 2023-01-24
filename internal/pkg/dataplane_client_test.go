package pkg

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestPostConfig(t *testing.T) {
	tc := &http.Client{Transport: RoundTripFunc(func(req *http.Request) *http.Response {
		_, _, ok := req.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth to be set, got", ok)
		}
		if !strings.Contains(req.URL.String(), "services/haproxy/configuration/raw?skip_version=true") {
			t.Error("expected request to contain /services/haproxy/configuration/raw?skip_version=true, got", req.URL.String())
		}
		if req.Method != "POST" {
			t.Error("expected request method to be POST, got", req.Method)
		}
		if req.Header.Get("Content-Type") != "text/plain" {
			t.Error("expected request Content-Type header to be text//plain, got", req.Header.Get("Content-Type"))
		}

		return &http.Response{
			StatusCode: http.StatusAccepted,
		}
	})}

	dc := DataPlaneClient{
		client:  tc,
		baseURL: "http://localhost:5555/v2",
	}

	_ = dc.PostConfig(context.TODO(), "cfg")
}

func TestAPIIsReady(t *testing.T) {
	// test 200 response
	tcReady := &http.Client{Transport: RoundTripFunc(func(req *http.Request) *http.Response {
		_, _, ok := req.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth to be set, got", ok)
		}
		if req.Method != "GET" {
			t.Error("expected request method to be GET, got", req.Method)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
		}
	})}

	dc := DataPlaneClient{
		client:  tcReady,
		baseURL: "http://localhost:5555/v2",
	}

	ready := dc.apiIsReady(context.TODO())
	if !ready {
		t.Error("expected dataplane api readiness to be true, got:", ready)
	}

	// test non-200 response
	tcNotReady := &http.Client{Transport: RoundTripFunc(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusRequestTimeout,
		}
	})}

	dc = DataPlaneClient{
		client:  tcNotReady,
		baseURL: "http://localhost:5555/v2",
	}

	ready = dc.apiIsReady(context.TODO())
	if ready {
		t.Error("expected dataplane api readiness to be false, got:", ready)
	}
}
