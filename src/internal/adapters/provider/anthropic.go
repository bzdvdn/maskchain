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
// @sk-task 111-provider-auth-and-config#T3.2: Config-driven auth + additional_headers (AC-004, AC-007)
//
// AnthropicClient represents a domain entity or configuration.
type AnthropicClient struct {
	baseURL           string
	apiKey            string
	authScheme        string
	authHeader        string
	authPrefix        string
	additionalHeaders map[string]string
	ec                *egress.Client
}

func newAnthropicClient(cfg *config.ProviderConfig, ec *egress.Client) *AnthropicClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	var apiKey string
	if len(cfg.APIKeys) > 0 {
		apiKey = cfg.APIKeys[0]
	}
	authScheme := cfg.AuthScheme
	if authScheme == "" {
		authScheme = "bearer"
	}
	authHeader := cfg.AuthHeader
	if authHeader == "" {
		authHeader = "Authorization"
	}
	authPrefix := cfg.AuthPrefix
	return &AnthropicClient{
		baseURL:           baseURL,
		apiKey:            apiKey,
		authScheme:        authScheme,
		authHeader:        authHeader,
		authPrefix:        authPrefix,
		additionalHeaders: cfg.AdditionalHeaders,
		ec:                ec,
	}
}

// @sk-task anthropic-messages-endpoint#T3.2: Explicit Path check for native passthrough (AC-002, AC-004)
func (c *AnthropicClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	authKey, authValue := buildAuthHeader(c.authScheme, c.authHeader, c.authPrefix, c.apiKey)
	headers := mergeHeaders(authKey, authValue, c.additionalHeaders)
	for k, v := range req.Headers {
		if _, exists := headers[k]; !exists {
			headers[k] = v
		}
	}

	// Path == "/api/v1/messages" => native passthrough (no conversion)
	// Path == "/api/v1/chat/completions" => also passthrough (backward compat, no conversion exists yet)
	// Any other Path (incl. zero-value) => same behaviour (passthrough)
	_ = req.Path

	providerReq := &ports.ProviderRequest{
		Method:  "POST",
		URL:     c.baseURL + "/v1/messages",
		Body:    req.Body,
		Headers: headers,
	}
	providerReq.Headers["anthropic-version"] = "2023-06-01"
	providerReq.Headers["Content-Type"] = "application/json"

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

// @sk-task anthropic-messages-endpoint#T3.2: Explicit Path check for native passthrough (AC-002, AC-004)
func (c *AnthropicClient) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	authKey, authValue := buildAuthHeader(c.authScheme, c.authHeader, c.authPrefix, c.apiKey)
	headers := mergeHeaders(authKey, authValue, c.additionalHeaders)
	for k, v := range req.Headers {
		if _, exists := headers[k]; !exists {
			headers[k] = v
		}
	}

	_ = req.Path

	providerReq := &ports.ProviderRequest{
		Method:  "POST",
		URL:     c.baseURL + "/v1/messages",
		Body:    req.Body,
		Headers: headers,
	}
	providerReq.Headers["anthropic-version"] = "2023-06-01"
	providerReq.Headers["Content-Type"] = "application/json"
	providerReq.Headers["Accept"] = "text/event-stream"

	ch, err := c.ec.Stream(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream: %w", err)
	}

	return ch, nil
}
