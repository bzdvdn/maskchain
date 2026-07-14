package egress

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

var ErrCircuitBreakerOpen = errors.New("provider skipped by circuit breaker")

// @sk-task 71-egress-streaming#T2.1: Implement egress.Client with Call(), proxy, pool, timeout, cancellation (AC-001, AC-002, AC-004, AC-005)
// @sk-task 116-connection-pool-fixes#T2.2: Add timeout and circuit breaker fields (AC-002, AC-006, AC-007)
type Client struct {
	cfg     *config.EgressConfig
	tp      *http.Transport
	timeout time.Duration
	cb      *CircuitBreaker
}

func NewClient(cfg *config.EgressConfig) *Client {
	return &Client{
		cfg: cfg,
		tp:  NewTransport(cfg),
	}
}

// @sk-task 116-connection-pool-fixes#T2.2: Add NewClientWithTransport for per-provider isolation (AC-002, AC-008)
func NewClientWithTransport(cfg *config.EgressConfig, tp *http.Transport, timeout time.Duration, cb *CircuitBreaker) *Client {
	return &Client{
		cfg:     cfg,
		tp:      tp,
		timeout: timeout,
		cb:      cb,
	}
}

// @sk-task 116-connection-pool-fixes#T2.2: Add circuit breaker check and per-provider timeout to Call (AC-002, AC-006)
// @sk-task 116-connection-pool-fixes#T3.4: Wire cb.Fail/cb.Reset in Call (AC-006, AC-007)
func (c *Client) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	if c.cb != nil && !c.cb.Allow() {
		return nil, ErrCircuitBreakerOpen
	}

	var cancel context.CancelFunc
	ctx, cancel = c.withTimeout(ctx)
	defer cancel()

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
		if c.cb != nil {
			c.cb.Fail()
		}
		return nil, err
	}
	defer resp.Body.Close()

	if c.cb != nil {
		c.cb.Reset()
	}

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
// @sk-task 116-connection-pool-fixes#T2.2: Add circuit breaker check and per-provider timeout to Stream (AC-002, AC-006)
func (c *Client) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	if c.cb != nil && !c.cb.Allow() {
		return nil, ErrCircuitBreakerOpen
	}

	streamCtx, cancel := c.withTimeout(ctx)
	ch, err := c.streamSSE(streamCtx, req, cancel)
	if err != nil {
		cancel()
		return nil, err
	}

	return ch, nil
}

// @sk-task 116-connection-pool-fixes#T2.2: Apply per-provider timeout if ctx has no deadline (AC-002)
func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}

const defaultDialTimeout = 30 * time.Second
