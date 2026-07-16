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

// @sk-task ollama-provider#T2.1: Implement OllamaClient with Call and Stream (AC-002, AC-003, AC-004)
type OllamaClient struct {
	baseURL           string
	apiKey            string
	authScheme        string
	authHeader        string
	authPrefix        string
	additionalHeaders map[string]string
	ec                *egress.Client
}

func newOllamaClient(cfg *config.ProviderConfig, ec *egress.Client) *OllamaClient {
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
	return &OllamaClient{
		baseURL:           baseURL,
		apiKey:            apiKey,
		authScheme:        authScheme,
		authHeader:        authHeader,
		authPrefix:        authPrefix,
		additionalHeaders: cfg.AdditionalHeaders,
		ec:                ec,
	}
}

// @sk-task ollama-provider#T2.1: Implement OllamaClient with Call and Stream (AC-002, AC-003, AC-004)
func (c *OllamaClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	providerReq := c.buildRequest(req)
	providerReq.Headers["Content-Type"] = "application/json"

	resp, err := c.ec.Call(ctx, providerReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		perr := ParseProviderError(resp.StatusCode, resp.Body, "ollama")
		resp.Body, _ = json.Marshal(perr)
		return resp, nil
	}

	return resp, nil
}

// @sk-task ollama-provider#T2.1: Implement OllamaClient with Call and Stream (AC-002, AC-003, AC-004)
func (c *OllamaClient) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	providerReq := c.buildRequest(req)
	providerReq.Headers["Content-Type"] = "application/json"
	providerReq.Headers["Accept"] = "text/event-stream"

	ch, err := c.ec.Stream(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: %w", err)
	}

	return ch, nil
}

func (c *OllamaClient) buildRequest(req *ports.ProviderRequest) *ports.ProviderRequest {
	headers := make(map[string]string)
	for k, v := range c.additionalHeaders {
		headers[k] = v
	}
	for k, v := range req.Headers {
		if _, exists := headers[k]; !exists {
			headers[k] = v
		}
	}
	if c.apiKey != "" {
		authKey, authValue := buildAuthHeader(c.authScheme, c.authHeader, c.authPrefix, c.apiKey)
		headers[authKey] = authValue
	}
	return &ports.ProviderRequest{
		Method:  "POST",
		URL:     c.baseURL + "/v1/chat/completions",
		Body:    req.Body,
		Headers: headers,
	}
}
