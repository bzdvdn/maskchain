package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

func writeSSE(w io.Writer, data string) {
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// @sk-task 112-proxy-streaming-wiring#T2.1: Add Stream bool to chatRequest (AC-001)
type chatRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

// @sk-task 70-routing-engine#T3.1: Implement routing proxy handler (AC-003, AC-004)
// @sk-task 118-api-consistency#T2.2: Set skipEnvelope on proxy raw body responses (AC-001, AC-002)
type RoutingProxyHandler struct {
	selector *routingSvc.RouteSelector
	fallback *routingSvc.FallbackHandler
}

func NewRoutingProxyHandler(selector *routingSvc.RouteSelector, fallback *routingSvc.FallbackHandler) *RoutingProxyHandler {
	return &RoutingProxyHandler{selector: selector, fallback: fallback}
}

func (h *RoutingProxyHandler) HandleChatCompletion(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
		return
	}

	// @sk-task 80-tenant-isolation#T2.4: Read tenant from auth middleware context (AC-006)
	tCtx, _ := middleware.TenantFromContext(c)
	tenantID := ""
	if tCtx != nil {
		tenantID = tCtx.Slug().String()
	}
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}

	firstProvider, providers, err := h.selector.Select(req.Model, tenantID)
	if errors.Is(err, routingSvc.ErrNoRoute) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no route for model " + req.Model})
		return
	}

	if firstProvider != nil {
		// @sk-task 80-tenant-isolation#T3.1: Propagate X-Tenant-ID to upstream (AC-007)
		providerReq := &ports.ProviderRequest{
			Method: http.MethodPost,
			URL:    "/v1/chat/completions",
			Body:   body,
			Headers: map[string]string{
				"X-Tenant-ID": tenantID,
			},
		}

		if req.Stream {
			// @sk-task 112-proxy-streaming-wiring#T3.1: Streaming branch first provider (AC-003, AC-004, AC-005)
			h.streamFromProvider(c, providerReq, []string{firstProvider.Name})
			return
		}

		resp, providerName, fbErr := h.fallback.Call(c.Request.Context(), []string{firstProvider.Name}, providerReq)
		if fbErr == nil && resp != nil {
			c.Header("X-Provider", providerName)
			for k, v := range resp.Headers {
				c.Header(k, v)
			}
			c.Set(middleware.SkipEnvelopeKey, true)
			c.Data(resp.StatusCode, "application/json", resp.Body)
			return
		}
	}

	// First provider failed or none healthy — try fallback chain
	if len(providers) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no healthy provider for model " + req.Model})
		return
	}

	providerReq := &ports.ProviderRequest{
		Method: http.MethodPost,
		URL:    "/v1/chat/completions",
		Body:   body,
	}
	if req.Stream {
		// @sk-task 112-proxy-streaming-wiring#T3.1: Streaming branch fallback chain (AC-003, AC-006)
		h.streamFromProvider(c, providerReq, providers)
		return
	}

	resp, providerName, fbErr := h.fallback.Call(c.Request.Context(), providers, providerReq)
	if fbErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no healthy provider for model " + req.Model})
		return
	}

	c.Header("X-Provider", providerName)
	for k, v := range resp.Headers {
		c.Header(k, v)
	}
	c.Set(middleware.SkipEnvelopeKey, true)
	c.Data(resp.StatusCode, "application/json", resp.Body)
}

// @sk-task 112-proxy-streaming-wiring#T3.1: Stream from provider via SSE (AC-003, AC-004, AC-005)
func (h *RoutingProxyHandler) streamFromProvider(c *gin.Context, providerReq *ports.ProviderRequest, providers []string) {
	ch, providerName, err := h.fallback.Stream(c.Request.Context(), providers, providerReq)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no healthy provider for model"})
		return
	}

	c.Header("X-Provider", providerName)
	c.Set(middleware.SkipEnvelopeKey, true)

	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case chunk, ok := <-ch:
			if !ok {
				return false
			}
			if chunk.Err != nil {
				writeSSE(w, fmt.Sprintf(`{"error":{"message":"%s"}}`, chunk.Err.Error()))
				return false
			}
			if chunk.Done {
				writeSSE(w, "[DONE]")
				return false
			}
			writeSSE(w, string(chunk.Data))
			return true
		}
	})
}

func ProxyChatCompletionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"choices": []gin.H{
			{
				"message": gin.H{
					"role":    "assistant",
					"content": "ok",
				},
			},
		},
	})
}

func ProxyCompletionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"choices": []gin.H{
			{
				"text": "ok",
			},
		},
	})
}
