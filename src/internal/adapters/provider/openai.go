package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task 110-provider-adapters#T3.1: Implement OpenAIClient with Call and Stream (AC-002, AC-003)
type OpenAIClient struct {
	baseURL string
	apiKey  string
	ec      *egress.Client
}

func newOpenAIClient(cfg *config.ProviderConfig, ec *egress.Client) *OpenAIClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	return &OpenAIClient{
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
		ec:      ec,
	}
}

func (c *OpenAIClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	providerReq := &ports.ProviderRequest{
		Method: "POST",
		URL:    c.baseURL + "/v1/chat/completions",
		Body:   req.Body,
		Headers: map[string]string{
			"Authorization": "Bearer " + c.apiKey,
			"Content-Type":  "application/json",
		},
	}

	resp, err := c.ec.Call(ctx, providerReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		perr := ParseProviderError(resp.StatusCode, resp.Body, "openai")
		resp.Body, _ = json.Marshal(perr)
		return resp, nil
	}

	return resp, nil
}

func (c *OpenAIClient) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	providerReq := &ports.ProviderRequest{
		Method: "POST",
		URL:    c.baseURL + "/v1/chat/completions",
		Body:   req.Body,
		Headers: map[string]string{
			"Authorization": "Bearer " + c.apiKey,
			"Content-Type":  "application/json",
			"Accept":        "text/event-stream",
		},
	}

	ch, err := c.ec.Stream(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	return ch, nil
}
