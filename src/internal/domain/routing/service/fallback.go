package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task 70-routing-engine#T2.3: Implement FallbackHandler (AC-002, AC-007)
type FallbackHandler struct {
	clients map[string]ports.ProviderClient
}

func NewFallbackHandler(clients map[string]ports.ProviderClient) *FallbackHandler {
	return &FallbackHandler{clients: clients}
}

func (h *FallbackHandler) Call(ctx context.Context, providers []string, req *ports.ProviderRequest) (*ports.ProviderResponse, string, error) {
	var lastErr error
	for _, name := range providers {
		client, ok := h.clients[name]
		if !ok {
			lastErr = fmt.Errorf("provider %s not configured", name)
			continue
		}
		resp, err := client.Call(ctx, req)
		if err != nil {
			if isRetriableError(err) {
				lastErr = err
				continue
			}
			return resp, name, err
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("provider returned status %d", resp.StatusCode)
			continue
		}
		return resp, name, nil
	}
	return nil, "", lastErr
}

// @sk-task 112-proxy-streaming-wiring#T1.1: Implement FallbackHandler.Stream() (AC-006)
func (h *FallbackHandler) Stream(ctx context.Context, providers []string, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, string, error) {
	var lastErr error
	for _, name := range providers {
		client, ok := h.clients[name]
		if !ok {
			lastErr = fmt.Errorf("provider %s not configured", name)
			continue
		}
		ch, err := client.Stream(ctx, req)
		if err != nil {
			if isRetriableError(err) {
				lastErr = err
				continue
			}
			return ch, name, err
		}
		return ch, name, nil
	}
	ch := make(chan ports.ProviderChunk, 1)
	ch <- ports.ProviderChunk{Err: lastErr, Done: true}
	close(ch)
	return ch, "", nil
}

func isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if strings.Contains(err.Error(), "connection refused") {
		return true
	}
	if strings.Contains(err.Error(), "timeout") {
		return true
	}
	return false
}
