package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-test 110-provider-adapters#T3.2: TestOpenAIClient_Call (AC-002)
func TestOpenAIClient_Call(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Authorization", "Bearer sk-test-key")
		assertHeader(t, r, "Content-Type", "application/json")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Hello!"}}]}`))
	})
	defer srv.Close()

	client := newTestOpenAI(t, srv.URL, "sk-test-key")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
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
	if len(result.Choices) == 0 || result.Choices[0].Message.Content != "Hello!" {
		t.Errorf("expected content=Hello!, got %+v", result)
	}
}

// @sk-test 110-provider-adapters#T3.2: TestOpenAIClient_Stream (AC-003)
func TestOpenAIClient_Stream(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Authorization", "Bearer sk-test-key")
		assertHeader(t, r, "Accept", "text/event-stream")

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	client := newTestOpenAI(t, srv.URL, "sk-test-key")
	ch, err := client.Stream(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var chunks []string
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

// @sk-test 110-provider-adapters#T4.2: TestAnthropicClient_Call (AC-004)
func TestAnthropicClient_Call(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "x-api-key", "sk-ant-test-key")
		assertHeader(t, r, "anthropic-version", "2023-06-01")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"Hello from Claude"}]}`))
	})
	defer srv.Close()

	client := newTestAnthropic(t, srv.URL, "sk-ant-test-key")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"claude-3","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(result.Content) == 0 || result.Content[0].Text != "Hello from Claude" {
		t.Errorf("expected text=Hello from Claude, got %+v", result)
	}
}

// @sk-test 110-provider-adapters#T4.2: TestAnthropicClient_Stream (AC-006)
func TestAnthropicClient_Stream(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "x-api-key", "sk-ant-test-key")

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"Hello\"}}\n\n")
		_, _ = fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\" world\"}}\n\n")
		_, _ = fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	})
	defer srv.Close()

	client := newTestAnthropic(t, srv.URL, "sk-ant-test-key")
	ch, err := client.Stream(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"claude-3","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var chunks []string
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("unexpected error: %v", chunk.Err)
		}
		if chunk.Done {
			break
		}
		chunks = append(chunks, string(chunk.Data))
	}

	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks (2 content + 1 stop), got %d: %v", len(chunks), chunks)
	}
}

// @sk-test 110-provider-adapters#T5.1: TestProviderClient_Factory (AC-001)
func TestProviderClient_Factory(t *testing.T) {
	egressCfg := &config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Second,
	}

	client, err := NewProviderClient(&config.ProviderConfig{
		Name:    "test-openai",
		APIType: "openai",
		BaseURL: "https://api.example.com",
		APIKeys: []string{"sk-test"},
	}, egressCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("expected *OpenAIClient, got %T", client)
	}

	client, err = NewProviderClient(&config.ProviderConfig{
		Name:       "test-anthropic",
		APIType:    "anthropic",
		BaseURL:    "https://api.anthropic.com",
		APIKeys:    []string{"sk-ant-test"},
		AuthScheme: "api-key",
		AuthHeader: "x-api-key",
	}, egressCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.(*AnthropicClient); !ok {
		t.Errorf("expected *AnthropicClient, got %T", client)
	}
}

// @sk-test 110-provider-adapters#T5.1: TestProviderClient_FactoryUnknownType (AC-001)
func TestProviderClient_FactoryUnknownType(t *testing.T) {
	_, err := NewProviderClient(&config.ProviderConfig{
		Name:    "test-unknown",
		APIType: "unknown",
		BaseURL: "https://api.example.com",
	}, &config.EgressConfig{})
	if err == nil {
		t.Fatal("expected error for unknown api_type, got nil")
	}
}

// @sk-test 110-provider-adapters#T5.1: TestProviderClient_FactoryEmptyType (AC-001)
func TestProviderClient_FactoryEmptyType(t *testing.T) {
	_, err := NewProviderClient(&config.ProviderConfig{
		Name:    "test-empty",
		APIType: "",
		BaseURL: "https://api.example.com",
	}, &config.EgressConfig{})
	if err == nil {
		t.Fatal("expected error for empty api_type, got nil")
	}
}

// @sk-test 110-provider-adapters#T3.2: TestOpenAIClient_Error (AC-005)
func TestOpenAIClient_Error(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"Invalid API key"}}`))
	})
	defer srv.Close()

	client := newTestOpenAI(t, srv.URL, "sk-wrong")
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

// @sk-test 110-provider-adapters#T4.2: TestAnthropicClient_Error (AC-005)
func TestAnthropicClient_Error(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"invalid_request_error","message":"Invalid request"}`))
	})
	defer srv.Close()

	client := newTestAnthropic(t, srv.URL, "sk-ant-wrong")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"claude-3"}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	var perr ProviderError
	if err := json.Unmarshal(resp.Body, &perr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if perr.Type != "invalid_request_error" || perr.Message != "Invalid request" {
		t.Errorf("unexpected error: %+v", perr)
	}
}

// --- test helpers ---

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(handler))
}

// @sk-test 111-provider-auth-and-config#T3.3: Updated test helper for APIKeys (AC-004, AC-007)
func newTestOpenAI(t *testing.T, baseURL, apiKey string) *OpenAIClient {
	t.Helper()
	ec := egress.NewClient(&config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Second,
	})
	return newOpenAIClient(&config.ProviderConfig{
		Name:    "test-openai",
		BaseURL: baseURL,
		APIKeys: []string{apiKey},
	}, ec)
}

// @sk-test 111-provider-auth-and-config#T3.3: Updated test helper for APIKeys + auth config (AC-004, AC-007)
func newTestAnthropic(t *testing.T, baseURL, apiKey string) *AnthropicClient {
	t.Helper()
	ec := egress.NewClient(&config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Second,
	})
	return newAnthropicClient(&config.ProviderConfig{
		Name:       "test-anthropic",
		BaseURL:    baseURL,
		APIKeys:    []string{apiKey},
		AuthScheme: "api-key",
		AuthHeader: "x-api-key",
	}, ec)
}

// @sk-test 111-provider-auth-and-config#T4.2: TestProviderClient_AuthHeader — кастомный заголовок (AC-004)
func TestProviderClient_AuthHeader(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "X-API-Key", "sk-custom")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()

	ec := egress.NewClient(&config.EgressConfig{MaxIdleConns: 1, IdleTimeout: time.Second})
	client := newOpenAIClient(&config.ProviderConfig{
		Name:       "test",
		BaseURL:    srv.URL,
		APIKeys:    []string{"sk-custom"},
		AuthScheme: "api-key",
		AuthHeader: "X-API-Key",
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

// @sk-test 111-provider-auth-and-config#T4.2: TestProviderClient_AdditionalHeaders — произвольные заголовки (AC-007)
func TestProviderClient_AdditionalHeaders(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Authorization", "Bearer sk-key")
		assertHeader(t, r, "X-Org-Id", "acme")
		assertHeader(t, r, "X-Custom", "value")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()

	ec := egress.NewClient(&config.EgressConfig{MaxIdleConns: 1, IdleTimeout: time.Second})
	client := newOpenAIClient(&config.ProviderConfig{
		Name:              "test",
		BaseURL:           srv.URL,
		APIKeys:           []string{"sk-key"},
		AuthScheme:        "bearer",
		AuthHeader:        "Authorization",
		AdditionalHeaders: map[string]string{"X-Org-Id": "acme", "X-Custom": "value"},
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

func assertHeader(t *testing.T, r *http.Request, key, expected string) {
	t.Helper()
	if got := r.Header.Get(key); got != expected {
		t.Errorf("expected header %s=%q, got %q", key, expected, got)
	}
}

// @sk-test custom-auth-prefix: TestProviderClient_CustomAuthPrefix — кастомный префикс (AC-004, AC-007)
func TestProviderClient_CustomAuthPrefix(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "X-OAuth", "Token sk-custom-token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()

	ec := egress.NewClient(&config.EgressConfig{MaxIdleConns: 1, IdleTimeout: time.Second})
	client := newOpenAIClient(&config.ProviderConfig{
		Name:       "test",
		BaseURL:    srv.URL,
		APIKeys:    []string{"sk-custom-token"},
		AuthScheme: "bearer",
		AuthHeader: "X-OAuth",
		AuthPrefix: "Token ",
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
