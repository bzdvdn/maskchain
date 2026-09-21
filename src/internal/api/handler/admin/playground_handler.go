package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
)

// @sk-task ui-playground: PlaygroundHandler proxies test chat completions to the gateway.
//
// The admin UI cannot safely hold a tenant key across origins, so it asks the
// admin process to relay a single test request to the data plane. The gateway
// URL comes from admin.gateway_url.
type PlaygroundHandler struct {
	gatewayURL string
	client     *http.Client
}

// NewPlaygroundHandler builds the handler. An empty gatewayURL falls back to the
// local default.
func NewPlaygroundHandler(gatewayURL string) *PlaygroundHandler {
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8080"
	}
	return &PlaygroundHandler{
		gatewayURL: strings.TrimRight(gatewayURL, "/"),
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

type playgroundMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type playgroundRequest struct {
	APIKey   string              `json:"api_key" binding:"required"`
	Model    string              `json:"model" binding:"required"`
	Messages []playgroundMessage `json:"messages" binding:"required"`
}

// Handle forwards a single non-streaming chat completion through the gateway
// shield/routing pipeline and echoes the upstream status, headers and body.
func (h *PlaygroundHandler) Handle(c *gin.Context) {
	var req playgroundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}
	if len(req.Messages) == 0 {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "messages is required")
		return
	}

	payload, err := json.Marshal(gin.H{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
	})
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to encode request")
		return
	}

	httpReq, err := http.NewRequestWithContext(
		c.Request.Context(),
		http.MethodPost,
		h.gatewayURL+"/api/v1/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to build gateway request")
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"status": http.StatusBadGateway,
			"body":   gin.H{"error": "gateway unreachable", "detail": err.Error()},
		})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		body = string(raw)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        resp.StatusCode,
		"shield_status": resp.Header.Get("X-Shield-Status"),
		"body":          body,
	})
}
