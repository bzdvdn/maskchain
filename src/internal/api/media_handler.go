package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task media-endpoints#T2.1: media passthrough handler (AC-001, AC-003, AC-005, AC-010)
//
// MediaHandler proxies image/audio endpoints. It forwards the raw request body
// (JSON or multipart) and the original Content-Type to the selected provider and
// returns the provider response with its status and Content-Type. The body is
// never re-encoded, and no response envelope or SSE wrapping is applied.
type MediaHandler struct {
	selector *routingSvc.RouteSelector
	fallback *routingSvc.FallbackHandler
}

func NewMediaHandler(selector *routingSvc.RouteSelector, fallback *routingSvc.FallbackHandler) *MediaHandler {
	return &MediaHandler{selector: selector, fallback: fallback}
}

func (h *MediaHandler) HandleMedia(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	contentType := c.GetHeader("Content-Type")
	model, err := middleware.ExtractModel(contentType, body)
	if err != nil || model == "" {
		respondWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "model is required")
		return
	}

	tCtx, _ := middleware.TenantFromContext(c)
	tenantID := ""
	if tCtx != nil {
		tenantID = tCtx.Slug().String()
	}
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}

	_, providers, selErr := h.selector.Select(model, tenantID)
	if errors.Is(selErr, routingSvc.ErrNoRoute) {
		respondWithError(c, http.StatusBadRequest, "NO_ROUTE", "no route for model "+model)
		return
	}
	if len(providers) == 0 {
		respondWithError(c, http.StatusServiceUnavailable, "NO_HEALTHY_PROVIDER", "no healthy provider for model "+model)
		return
	}

	upstreamPath := strings.TrimPrefix(c.Request.URL.Path, "/api")
	providerReq := &ports.ProviderRequest{
		Method:      http.MethodPost,
		URL:         upstreamPath,
		Body:        body,
		Path:        c.Request.URL.Path,
		RawQuery:    c.Request.URL.RawQuery,
		ContentType: contentType,
		Headers: map[string]string{
			"X-Tenant-ID": tenantID,
		},
	}

	resp, providerName, fbErr := h.fallback.Call(c.Request.Context(), providers, providerReq)
	if fbErr != nil {
		if errors.Is(fbErr, ports.ErrUnsupportedEndpoint) {
			respondUnsupportedEndpoint(c, providers, upstreamPath)
			return
		}
		respondWithError(c, http.StatusServiceUnavailable, "NO_HEALTHY_PROVIDER", "no healthy provider for model "+model)
		return
	}
	if resp == nil {
		respondWithError(c, http.StatusServiceUnavailable, "NO_HEALTHY_PROVIDER", "no healthy provider for model "+model)
		return
	}

	c.Header("X-Provider", providerName)
	forwardUpstreamHeaders(c, resp.Headers)
	c.Set(middleware.SkipEnvelopeKey, true)

	outContentType := headerValue(resp.Headers, "content-type")
	if outContentType == "" {
		outContentType = "application/octet-stream"
	}
	c.Data(resp.StatusCode, outContentType, resp.Body)
}

// headerValue looks up a header case-insensitively.
func headerValue(headers map[string]string, key string) string {
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}
