package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-test provider-adapters-expansion#T2.2: TestProxyClient_Call — auth header set, body forwarded (AC-007)
func TestProxyClient_Call(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Authorization", "Bearer sk-proxy-key")
		assertHeader(t, r, "Content-Type", "application/json")
		assertHeader(t, r, "X-Tenant-ID", "test-tenant")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Hello from proxy"}}]}`))
	})
	defer srv.Close()

	client := newTestProxy(t, srv.URL, "sk-proxy-key")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
		Headers: map[string]string{
			"X-Tenant-ID": "test-tenant",
		},
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content != "Hello from proxy" {
		t.Errorf("expected content=Hello from proxy, got %+v", result)
	}
}

// @sk-test provider-adapters-expansion#T2.2: TestProxyClient_Stream — SSE passthrough (AC-007)
func TestProxyClient_Stream(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Authorization", "Bearer sk-proxy-key")
		assertHeader(t, r, "Accept", "text/event-stream")

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	client := newTestProxy(t, srv.URL, "sk-proxy-key")
	ch, err := client.Stream(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var chunks []string //nolint:prealloc // unknown size until stream completes
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("unexpected error: %v", chunk.Err)
		}
		if chunk.Done {
			break
		}
		chunks = append(chunks, string(chunk.Data))
	}

	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
}

// @sk-test provider-adapters-expansion#T2.2: TestProxyClient_NoAuthLeak — tenant Authorization not forwarded (AC-007)
func TestProxyClient_NoAuthLeak(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Tenant's Authorization must NOT be present; proxy uses its own api_key
		if r.Header.Get("Authorization") != "Bearer sk-proxy-key" {
			t.Error("expected proxy Authorization header, got none")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()

	client := newTestProxy(t, srv.URL, "sk-proxy-key")
	// Simulate a ProviderRequest that carries a tenant auth header
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gpt-4"}`),
		Headers: map[string]string{
			"Authorization": "Bearer sk-tenant-key", // MUST NOT reach upstream
		},
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// @sk-test provider-adapters-expansion#T2.2: TestProxyClient_EmptyAPIKey — no auth header sent (AC-007)
func TestProxyClient_EmptyAPIKey(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("Authorization"); v != "" {
			t.Errorf("expected no Authorization header for empty api_key, got %q", v)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()

	client := newTestProxy(t, srv.URL, "")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gpt-4"}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// @sk-test provider-adapters-expansion#T2.2: TestProxyClient_Error — proxy error format (AC-007)
func TestProxyClient_Error(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"Invalid API key"}}`))
	})
	defer srv.Close()

	client := newTestProxy(t, srv.URL, "sk-wrong")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gpt-4"}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	var perr ProviderError
	if err := json.Unmarshal(resp.Body, &perr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if perr.Type != "authentication_error" || perr.Message != "Invalid API key" {
		t.Errorf("unexpected error: %+v", perr)
	}
}

// @sk-test provider-adapters-expansion#T2.2: TestProxyClient_AdditionalHeaders — custom headers forwarded (AC-007)
func TestProxyClient_AdditionalHeaders(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Authorization", "Bearer sk-key")
		assertHeader(t, r, "X-Org-Id", "acme")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()

	ec := egress.NewClient(&config.EgressConfig{MaxIdleConns: 1, IdleTimeout: time.Second})
	client := newProxyClient(&config.ProviderConfig{
		Name:              "test",
		BaseURL:           srv.URL,
		APIKeys:           []string{"sk-key"},
		AuthScheme:        "bearer",
		AuthHeader:        "Authorization",
		AdditionalHeaders: map[string]string{"X-Org-Id": "acme"},
	}, ec)

	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gpt-4"}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func newTestProxy(t *testing.T, baseURL, apiKey string) *ProxyClient {
	t.Helper()
	ec := egress.NewClient(&config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Second,
	})
	return newProxyClient(&config.ProviderConfig{
		Name:    "test-proxy",
		BaseURL: baseURL,
		APIKeys: []string{apiKey},
	}, ec)
}
