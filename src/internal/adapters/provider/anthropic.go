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

// @sk-task 110-provider-adapters#T4.1: Implement AnthropicClient with Call and Stream (AC-004, AC-006)
type AnthropicClient struct {
	baseURL string
	apiKey  string
	ec      *egress.Client
}

func newAnthropicClient(cfg *config.ProviderConfig, ec *egress.Client) *AnthropicClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	return &AnthropicClient{
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
		ec:      ec,
	}
}

func (c *AnthropicClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	providerReq := &ports.ProviderRequest{
		Method: "POST",
		URL:    c.baseURL + "/v1/messages",
		Body:   req.Body,
		Headers: map[string]string{
			"x-api-key":         c.apiKey,
			"anthropic-version": "2023-06-01",
			"Content-Type":      "application/json",
		},
	}

	resp, err := c.ec.Call(ctx, providerReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		perr := ParseProviderError(resp.StatusCode, resp.Body, "anthropic")
		resp.Body, _ = json.Marshal(perr)
		return resp, nil
	}

	return resp, nil
}

func (c *AnthropicClient) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	providerReq := &ports.ProviderRequest{
		Method: "POST",
		URL:    c.baseURL + "/v1/messages",
		Body:   req.Body,
		Headers: map[string]string{
			"x-api-key":         c.apiKey,
			"anthropic-version": "2023-06-01",
			"Content-Type":      "application/json",
			"Accept":            "text/event-stream",
		},
	}

	ch, err := c.ec.Stream(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream: %w", err)
	}

	return ch, nil
}
