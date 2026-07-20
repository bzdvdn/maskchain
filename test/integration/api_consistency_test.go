//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func serverURL() string {
	url := os.Getenv("SERVER_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	return strings.TrimRight(url, "/")
}

var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func TestAPIEnvelope(t *testing.T) {
	url := serverURL()

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		headers  map[string]string
		wantCode int
		check    func(t *testing.T, body []byte)
		client   *http.Client
	}{
		{
			name:     "GET /api/v1/profiles",
			method:   http.MethodGet,
			path:     "/api/v1/profiles",
			wantCode: http.StatusNotFound,
			check:    expectErrorEnvelope,
		},
		{
			name:     "POST /api/v1/profiles",
			method:   http.MethodPost,
			path:     "/api/v1/profiles",
			body:     `{"slug":"test","name":"Test","description":"test"}`,
			wantCode: http.StatusNotFound,
			check:    expectErrorEnvelope,
			headers:  map[string]string{"Content-Type": "application/json"},
		},
		{
			name:     "GET /api/v1/incidents",
			method:   http.MethodGet,
			path:     "/api/v1/incidents",
			wantCode: http.StatusNotFound,
			check:    expectErrorEnvelope,
		},
		{
			name:     "POST /api/v1/shield/mask",
			method:   http.MethodPost,
			path:     "/api/v1/shield/mask",
			body:     "test text",
			wantCode: http.StatusOK,
			headers:  map[string]string{"Content-Type": "text/plain"},
		},
		{
			name:     "Redirect /v1/chat/completions",
			method:   http.MethodPost,
			path:     "/v1/chat/completions",
			wantCode: http.StatusMovedPermanently,
			client:   noRedirectClient,
		},
		{
			name:     "Redirect /v1/completions",
			method:   http.MethodPost,
			path:     "/v1/completions",
			wantCode: http.StatusMovedPermanently,
			client:   noRedirectClient,
		},
		{
			name:     "404 envelope",
			method:   http.MethodGet,
			path:     "/api/v1/nonexistent",
			wantCode: http.StatusNotFound,
			check:    expectErrorEnvelope,
		},
		{
			name:     "Health endpoint",
			method:   http.MethodGet,
			path:     "/health",
			wantCode: http.StatusOK,
		},
		{
			name:     "Liveness endpoint",
			method:   http.MethodGet,
			path:     "/live",
			wantCode: http.StatusOK,
		},
		{
			name:     "Readiness endpoint",
			method:   http.MethodGet,
			path:     "/ready",
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody io.Reader
			if tt.body != "" {
				reqBody = bytes.NewBufferString(tt.body)
			}

			req, err := http.NewRequest(tt.method, url+tt.path, reqBody)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			c := tt.client
			if c == nil {
				c = http.DefaultClient
			}
			resp, err := c.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantCode {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected status %d, got %d\nbody: %s", tt.wantCode, resp.StatusCode, string(body[:min(len(body), 300)]))
			}

			if tt.check != nil {
				body, _ := io.ReadAll(resp.Body)
				tt.check(t, body)
			}
		})
	}
}

func expectEnvelope(t *testing.T, body []byte) {
	t.Helper()
	var payload struct {
		Data  interface{} `json:"data"`
		Error interface{} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Errorf("expected envelope JSON, got parse error: %v\nbody: %s", err, string(body[:min(len(body), 200)]))
		return
	}
	if payload.Data == nil && payload.Error == nil {
		t.Errorf("expected envelope with data or error, got: %s", string(body[:min(len(body), 200)]))
	}
}

func expectErrorEnvelope(t *testing.T, body []byte) {
	t.Helper()
	var payload struct {
		Data  interface{} `json:"data"`
		Error interface{} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Errorf("expected error envelope JSON, got parse error: %v\nbody: %s", err, string(body[:min(len(body), 200)]))
		return
	}
	if payload.Error == nil {
		t.Errorf("expected error in envelope, got: %s", string(body[:min(len(body), 200)]))
	}
}

func TestVersionEndpoint(t *testing.T) {
	url := serverURL()
	req, _ := http.NewRequest(http.MethodGet, url+"/api/v1/version", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var payload struct {
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("expected version envelope: %v\nbody: %s", err, string(body[:min(len(body), 200)]))
	}
	if payload.Data.Version == "" {
		t.Errorf("expected non-empty version in response")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	url := serverURL()
	req, _ := http.NewRequest(http.MethodGet, url+"/metrics", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("expected text/plain content type, got %q", contentType)
	}
	if !bytes.Contains(body, []byte("maskchain_http_requests_total")) {
		t.Errorf("expected maskchain_http_requests_total metric in response")
	}
}
