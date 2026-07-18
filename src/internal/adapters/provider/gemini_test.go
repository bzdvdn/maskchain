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

// @sk-test provider-adapters-expansion#T3.2: TestGeminiClient_Call — OpenAI→Gemini conversion (AC-002)
func TestGeminiClient_Call(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Content-Type", "application/json")

		var geminiReq map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&geminiReq); err != nil {
			t.Fatalf("decode gemini request: %v", err)
		}

		// Verify OpenAI→Gemini conversion
		contents, ok := geminiReq["contents"].([]interface{})
		if !ok || len(contents) != 1 {
			t.Fatalf("expected 1 content, got %+v", geminiReq)
		}
		content := contents[0].(map[string]interface{})
		if content["role"] != "user" {
			t.Errorf("expected role=user, got %v", content["role"])
		}
		parts := content["parts"].([]interface{})
		if parts[0].(map[string]interface{})["text"] != "Hello" {
			t.Errorf("expected text=Hello, got %v", parts[0])
		}

		// Verify system instruction
		sysInstr, ok := geminiReq["systemInstruction"].(map[string]interface{})
		if !ok || sysInstr["text"] != "You are helpful" {
			t.Errorf("expected systemInstruction.text=You are helpful, got %v", sysInstr)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "Hi from Gemini"}]},
				"finishReason": "STOP"
			}]
		}`))
	})
	defer srv.Close()

	client := newTestGemini(t, srv.URL, "test-key")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gemini-2.0-flash","messages":[{"role":"system","content":"You are helpful"},{"role":"user","content":"Hello"}]}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify Gemini→OpenAI conversion
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resp.Body, &openAIResp); err != nil {
		t.Fatalf("unmarshal openai response: %v", err)
	}
	if len(openAIResp.Choices) != 1 || openAIResp.Choices[0].Message.Content != "Hi from Gemini" {
		t.Errorf("expected content=Hi from Gemini, got %+v", openAIResp)
	}
	if openAIResp.Choices[0].FinishReason == nil || *openAIResp.Choices[0].FinishReason != "stop" {
		t.Errorf("expected finish_reason=stop, got %v", openAIResp.Choices[0].FinishReason)
	}
}

// @sk-test provider-adapters-expansion#T3.2: TestGeminiClient_Stream — SSE streaming with conversion (AC-003)
func TestGeminiClient_Stream(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertHeader(t, r, "Content-Type", "application/json")

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]},\"finishReason\":null}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}]},\"finishReason\":null}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"\"}]},\"finishReason\":\"STOP\"}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	client := newTestGemini(t, srv.URL, "test-key")
	ch, err := client.Stream(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gemini-2.0-flash","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var contents []string
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("unexpected error: %v", chunk.Err)
		}
		if chunk.Done {
			break
		}
		var openAIChunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(chunk.Data, &openAIChunk); err != nil {
			t.Fatalf("unmarshal chunk: %v", err)
		}
		for _, c := range openAIChunk.Choices {
			contents = append(contents, c.Delta.Content)
		}
	}

	expected := []string{"Hello", " world"}
	if len(contents) < len(expected) {
		t.Errorf("expected at least %d text chunks, got %d: %v", len(expected), len(contents), contents)
	}
	for i := range expected {
		if i >= len(contents) {
			break
		}
		if contents[i] != expected[i] {
			t.Errorf("chunk %d: expected %q, got %q", i, expected[i], contents[i])
		}
	}
	// Last Gemini chunk with finishReason STOP may produce empty text — that's fine
	_ = contents
}

// @sk-test provider-adapters-expansion#T3.2: TestGeminiClient_Error — Gemini error format (AC-002)
func TestGeminiClient_Error(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"API key not valid"}}`))
	})
	defer srv.Close()

	client := newTestGemini(t, srv.URL, "bad-key")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gemini-2.0-flash","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}

	var perr ProviderError
	if err := json.Unmarshal(resp.Body, &perr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if perr.Message != "API key not valid" {
		t.Errorf("unexpected error message: %q", perr.Message)
	}
}

// @sk-test provider-adapters-expansion#T3.2: TestGeminiClient_NoSystem — no system prompt (AC-002)
func TestGeminiClient_NoSystem(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var geminiReq map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&geminiReq); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, ok := geminiReq["systemInstruction"]; ok {
			t.Error("expected no systemInstruction for user-only messages")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`))
	})
	defer srv.Close()

	client := newTestGemini(t, srv.URL, "key")
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"gemini-2.0-flash","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func newTestGemini(t *testing.T, baseURL, apiKey string) *GeminiClient {
	t.Helper()
	ec := egress.NewClient(&config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Second,
	})
	return newGeminiClient(&config.ProviderConfig{
		Name:    "test-gemini",
		BaseURL: baseURL,
		APIKeys: []string{apiKey},
	}, ec)
}
