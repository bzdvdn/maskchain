package egress

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task 71-egress-streaming#T2.1: Implement egress.Client with Call(), proxy, pool, timeout, cancellation (AC-001, AC-002, AC-004, AC-005)
type Client struct {
	cfg *config.EgressConfig
	tp  *http.Transport
}

func NewClient(cfg *config.EgressConfig) *Client {
	return &Client{
		cfg: cfg,
		tp:  newTransport(cfg),
	}
}

func (c *Client) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.doWithRetry(ctx, req.Method, func() (*http.Response, error) {
		return c.tp.RoundTrip(httpReq)
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &ports.ProviderResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    headers,
	}, nil
}

// @sk-task 71-egress-streaming#T2.1: Add Stream() to egress.Client for compatibility (will be implemented in T4.1)
// @sk-task 71-egress-streaming#T4.1: Wire real SSE streaming implementation (AC-003)
func (c *Client) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	return c.streamSSE(ctx, req)
}

const defaultDialTimeout = 30 * time.Second
