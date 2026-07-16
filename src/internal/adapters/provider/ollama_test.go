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

// @sk-test ollama-provider#T3.1: TestOllamaClient_ValidConfig (AC-001)
func TestOllamaClient_ValidConfig(t *testing.T) {
	ec := egress.NewClient(&config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Second,
	})
	client := newOllamaClient(&config.ProviderConfig{
		Name:    "test-ollama",
		APIType: "ollama",
		BaseURL: "http://localhost:11434",
	}, ec)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.baseURL != "http://localhost:11434" {
		t.Errorf("expected baseURL http://localhost:11434, got %s", client.baseURL)
	}
	if client.apiKey != "" {
		t.Errorf("expected empty apiKey, got %q", client.apiKey)
	}
}

// @sk-test ollama-provider#T3.1: TestOllamaClient_Call (AC-002)
func TestOllamaClient_Call(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Content-Type", "application/json")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Hello from Ollama!"}}]}`))
	})
	defer srv.Close()

	client := newTestOllama(t, srv.URL)
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}],"stream":false}`),
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
	if len(result.Choices) == 0 || result.Choices[0].Message.Content != "Hello from Ollama!" {
		t.Errorf("expected content=Hello from Ollama!, got %+v", result)
	}
}

// @sk-test ollama-provider#T3.1: TestOllamaClient_Stream (AC-003)
func TestOllamaClient_Stream(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Accept", "text/event-stream")

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	client := newTestOllama(t, srv.URL)
	ch, err := client.Stream(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}],"stream":true}`),
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

// @sk-test ollama-provider#T3.1: TestOllamaClient_NoAuthHeaders (AC-004)
func TestOllamaClient_NoAuthHeaders(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("Authorization"); v != "" {
			t.Errorf("expected no Authorization header, got %q", v)
		}
		if v := r.Header.Get("X-API-Key"); v != "" {
			t.Errorf("expected no X-API-Key header, got %q", v)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()

	client := newTestOllama(t, srv.URL)
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// @sk-test ollama-provider#T3.1: TestOllamaClient_Unreachable (AC-005)
func TestOllamaClient_Unreachable(t *testing.T) {
	client := newTestOllama(t, "http://127.0.0.1:1")
	_, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func newTestOllama(t *testing.T, baseURL string) *OllamaClient {
	t.Helper()
	ec := egress.NewClient(&config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Second,
	})
	return newOllamaClient(&config.ProviderConfig{
		Name:    "test-ollama",
		APIType: "ollama",
		BaseURL: baseURL,
	}, ec)
}
