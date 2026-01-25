package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
)

// TestServer is a lightweight integration test harness based on httptest.Server.
//
// It intentionally does not depend on application packages to avoid import cycles in unit tests.
// Tests can register handlers on Mux and issue requests via DoRequest.
type TestServer struct {
	Server *httptest.Server
	Client *http.Client
	Config *config.Config
	Mux    *http.ServeMux
}

// NewTestServer creates a new TestServer backed by an http.ServeMux.
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &TestServer{
		Server: srv,
		Client: srv.Client(),
		Config: nil,
		Mux:    mux,
	}
}

// DoRequest issues an HTTP request against the test server.
func (ts *TestServer) DoRequest(method, path string, body any) (*http.Response, error) {
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, ts.Server.URL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return ts.Client.Do(req)
}

// Cleanup closes the underlying server. It is safe to call multiple times.
func (ts *TestServer) Cleanup() {
	if ts.Server == nil {
		return
	}
	ts.Server.Close()
	ts.Server = nil
}
