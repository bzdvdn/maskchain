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
// @sk-task 111-provider-auth-and-config#T3.1: Config-driven auth + additional_headers (AC-004, AC-007)
type OpenAIClient struct {
	baseURL           string
	apiKey            string
	authScheme        string
	authHeader        string
	authPrefix        string
	additionalHeaders map[string]string
	ec                *egress.Client
}

func newOpenAIClient(cfg *config.ProviderConfig, ec *egress.Client) *OpenAIClient {
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
	return &OpenAIClient{
		baseURL:           baseURL,
		apiKey:            apiKey,
		authScheme:        authScheme,
		authHeader:        authHeader,
		authPrefix:        authPrefix,
		additionalHeaders: cfg.AdditionalHeaders,
		ec:                ec,
	}
}

func (c *OpenAIClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	authKey, authValue := buildAuthHeader(c.authScheme, c.authHeader, c.authPrefix, c.apiKey)
	headers := mergeHeaders(authKey, authValue, c.additionalHeaders)
	for k, v := range req.Headers {
		if _, exists := headers[k]; !exists {
			headers[k] = v
		}
	}
	providerReq := &ports.ProviderRequest{
		Method:  "POST",
		URL:     c.baseURL + "/v1/chat/completions",
		Body:    req.Body,
		Headers: headers,
	}
	providerReq.Headers["Content-Type"] = "application/json"

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
	authKey, authValue := buildAuthHeader(c.authScheme, c.authHeader, c.authPrefix, c.apiKey)
	headers := mergeHeaders(authKey, authValue, c.additionalHeaders)
	for k, v := range req.Headers {
		if _, exists := headers[k]; !exists {
			headers[k] = v
		}
	}
	providerReq := &ports.ProviderRequest{
		Method:  "POST",
		URL:     c.baseURL + "/v1/chat/completions",
		Body:    req.Body,
		Headers: headers,
	}
	providerReq.Headers["Content-Type"] = "application/json"
	providerReq.Headers["Accept"] = "text/event-stream"

	ch, err := c.ec.Stream(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	return ch, nil
}
