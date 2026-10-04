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
	providerReq, err := c.buildRequest(req)
	if err != nil {
		return nil, err
	}
	providerReq.Headers["Content-Type"] = contentTypeFor(req)

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
	providerReq, err := c.buildRequest(req)
	if err != nil {
		return nil, err
	}
	providerReq.Headers["Content-Type"] = contentTypeFor(req)
	providerReq.Headers["Accept"] = "text/event-stream"

	ch, err := c.ec.Stream(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("proxy stream: %w", err)
	}

	return ch, nil
}

// @sk-task media-endpoints#T1.2: honor an explicit Content-Type (multipart/binary) (AC-002, AC-003)
//
// contentTypeFor returns the outgoing Content-Type, defaulting to JSON. Media
// passthrough sets an explicit type (e.g. multipart/form-data with a boundary).
func contentTypeFor(req *ports.ProviderRequest) string {
	if req != nil && req.ContentType != "" {
		return req.ContentType
	}
	return "application/json"
}

func (c *ProxyClient) buildRequest(req *ports.ProviderRequest) (*ports.ProviderRequest, error) {
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
	// @sk-task provider-path-fidelity#T2.2: resolve upstream path from request (AC-001, AC-002, AC-003)
	upstreamURL, err := ResolveUpstreamURL("proxy", c.baseURL, req)
	if err != nil {
		return nil, err
	}
	return &ports.ProviderRequest{
		Method:  "POST",
		URL:     upstreamURL,
		Body:    req.Body,
		Headers: headers,
	}, nil
}
