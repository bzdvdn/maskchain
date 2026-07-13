package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

type chatRequest struct {
	Model string `json:"model"`
}

// @sk-task 70-routing-engine#T3.1: Implement routing proxy handler (AC-003, AC-004)
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
		resp, providerName, fbErr := h.fallback.Call(c.Request.Context(), []string{firstProvider.Name}, providerReq)
		if fbErr == nil && resp != nil {
			c.Header("X-Provider", providerName)
			for k, v := range resp.Headers {
				c.Header(k, v)
			}
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
	resp, providerName, fbErr := h.fallback.Call(c.Request.Context(), providers, providerReq)
	if fbErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no healthy provider for model " + req.Model})
		return
	}

	c.Header("X-Provider", providerName)
	for k, v := range resp.Headers {
		c.Header(k, v)
	}
	c.Data(resp.StatusCode, "application/json", resp.Body)
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
