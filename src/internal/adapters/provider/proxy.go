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

// @sk-task provider-adapters-expansion#T2.1: Create ProxyClient for generic OpenAI-compatible providers (AC-007)
//
// ProxyClient represents a domain entity or configuration.
type ProxyClient struct {
	baseURL           string
	apiKey            string
	authScheme        string
	authHeader        string
	authPrefix        string
	additionalHeaders map[string]string
	ec                *egress.Client
}

func newProxyClient(cfg *config.ProviderConfig, ec *egress.Client) *ProxyClient {
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
	return &ProxyClient{
		baseURL:           baseURL,
		apiKey:            apiKey,
		authScheme:        authScheme,
		authHeader:        authHeader,
		authPrefix:        authPrefix,
		additionalHeaders: cfg.AdditionalHeaders,
		ec:                ec,
	}
}

// @sk-task provider-adapters-expansion#T2.1: ProxyClient.Call — forward body with auth from config, no tenant header leak (AC-007)
func (c *ProxyClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	providerReq := c.buildRequest(req)
	providerReq.Headers["Content-Type"] = "application/json"

	resp, err := c.ec.Call(ctx, providerReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		perr := ParseProviderError(resp.StatusCode, resp.Body, "proxy")
		resp.Body, _ = json.Marshal(perr)
		return resp, nil
	}

	return resp, nil
}

// @sk-task provider-adapters-expansion#T2.1: ProxyClient.Stream — forward SSE stream with auth from config (AC-007)
func (c *ProxyClient) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	providerReq := c.buildRequest(req)
	providerReq.Headers["Content-Type"] = "application/json"
	providerReq.Headers["Accept"] = "text/event-stream"

	ch, err := c.ec.Stream(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("proxy stream: %w", err)
	}

	return ch, nil
}

func (c *ProxyClient) buildRequest(req *ports.ProviderRequest) *ports.ProviderRequest {
	headers := make(map[string]string)
	for k, v := range c.additionalHeaders {
		headers[k] = v
	}
	// Only forward X-Tenant-ID from the original request; never tenant auth headers
	if tid := req.Headers["X-Tenant-ID"]; tid != "" {
		headers["X-Tenant-ID"] = tid
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
