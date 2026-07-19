package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task 70-routing-engine#T2.3: Implement FallbackHandler (AC-002, AC-007)
type FallbackHandler struct {
	clients atomic.Pointer[map[string]ports.ProviderClient]
}

func NewFallbackHandler(clients map[string]ports.ProviderClient) *FallbackHandler {
	h := &FallbackHandler{}
	h.clients.Store(&clients)
	return h
}

// @sk-task config-hot-reload#T3.2: FallbackHandler.UpdateClients with atomic.Pointer (AC-005)
func (h *FallbackHandler) UpdateClients(clients map[string]ports.ProviderClient) {
	h.clients.Store(&clients)
}

func (h *FallbackHandler) clientsMap() map[string]ports.ProviderClient {
	m := h.clients.Load()
	if m == nil {
		return nil
	}
	return *m
}

func (h *FallbackHandler) Call(ctx context.Context, providers []string, req *ports.ProviderRequest) (*ports.ProviderResponse, string, error) {
	clients := h.clientsMap()
	var lastErr error
	for _, name := range providers {
		client, ok := clients[name]
		if !ok {
			lastErr = fmt.Errorf("provider %s not configured", name)
			slog.DebugContext(ctx, "fallback: provider not configured", "provider", name)
			continue
		}
		tStart := time.Now()
		resp, err := client.Call(ctx, req)
		elapsed := time.Since(tStart)
		if err != nil {
			slog.DebugContext(ctx, "fallback: provider call failed",
				"provider", name,
				"elapsed", elapsed.String(),
				"error", err.Error(),
				"retriable", isRetriableError(err),
			)
			if isRetriableError(err) {
				lastErr = err
				continue
			}
			return resp, name, err
		}
		slog.DebugContext(ctx, "fallback: provider succeeded",
			"provider", name,
			"elapsed", elapsed.String(),
			"status", resp.StatusCode,
		)
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("provider returned status %d", resp.StatusCode)
			slog.DebugContext(ctx, "fallback: provider returned server error, trying next",
				"provider", name, "status", resp.StatusCode,
			)
			continue
		}
		return resp, name, nil
	}
	slog.DebugContext(ctx, "fallback: all providers exhausted", "error", errString(lastErr))
	return nil, "", lastErr
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// @sk-task 112-proxy-streaming-wiring#T1.1: Implement FallbackHandler.Stream() (AC-006)
func (h *FallbackHandler) Stream(ctx context.Context, providers []string, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, string, error) {
	clients := h.clientsMap()
	var lastErr error
	for _, name := range providers {
		client, ok := clients[name]
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
