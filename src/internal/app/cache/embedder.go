package cacheapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// @sk-task semantic-cache-masked#T2.1: ExternalEmbedder calls OpenAI-compatible /embeddings (AC-001)
//
// ExternalEmbedder implements cache.Embedder by calling an OpenAI-compatible
// embeddings endpoint. It always embeds masked text only.
type ExternalEmbedder struct {
	client *http.Client
	url    string
}

func NewExternalEmbedder(url string, timeout time.Duration) *ExternalEmbedder {
	return &ExternalEmbedder{
		client: &http.Client{Timeout: timeout},
		url:    url,
	}
}

type embedRequest struct {
	Input string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// @sk-task semantic-cache-masked#T2.1: Embed maps masked text to a vector (AC-001)
func (e *ExternalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.url == "" {
		return nil, fmt.Errorf("embedder: external_url is empty")
	}
	body, err := json.Marshal(embedRequest{Input: text})
	if err != nil {
		return nil, fmt.Errorf("embedder: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedder: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedder: call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("embedder: endpoint returned %d", resp.StatusCode)
	}
	var parsed embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("embedder: decode response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("embedder: empty data in response")
	}
	return parsed.Data[0].Embedding, nil
}
