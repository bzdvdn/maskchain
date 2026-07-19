package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// @sk-task 131-analytics-pipeline#T3.1: Implement UsageMiddleware (AC-001, AC-003, AC-005)
type UsageMiddleware struct {
	registry *analytics.CostRateRegistry
	usageCh  chan<- analytics.TokenUsage
	log      *slog.Logger
}

func NewUsageMiddleware(registry *analytics.CostRateRegistry, usageCh chan<- analytics.TokenUsage, log *slog.Logger) *UsageMiddleware {
	return &UsageMiddleware{
		registry: registry,
		usageCh:  usageCh,
		log:      log,
	}
}

func (m *UsageMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		model := extractModel(c)
		if model == "" {
			c.Next()
			return
		}

		if isStreaming(c) {
			c.Next()
			return
		}

		w := &usageBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = w
		c.Next()

		if w.status != http.StatusOK {
			return
		}

		var resp struct {
			Usage *struct {
				PromptTokens     int64 `json:"prompt_tokens"`
				CompletionTokens int64 `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(w.body.Bytes(), &resp); err != nil {
			m.log.WarnContext(c.Request.Context(), "usage middleware: failed to parse response body", slog.String("error", err.Error()))
			return
		}

		if resp.Usage == nil {
			m.log.WarnContext(c.Request.Context(), "usage middleware: no usage field in response", slog.String("path", c.Request.URL.Path))
			return
		}

		tenantStr := extractTenantSlug(c)
		tenantID, _ := value.NewTenantID(tenantStr)
		cr := m.registry.Lookup(model)
		cost := cr.Cost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

		usage := analytics.TokenUsage{
			TenantID:     tenantID,
			Model:        model,
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			Cost:         cost,
			Timestamp:    time.Now(),
		}

		updateMetrics(tenantStr, model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, cost)

		select {
		case m.usageCh <- usage:
		default:
			m.log.WarnContext(c.Request.Context(), "usage middleware: usage channel full, dropping record")
		}
	}
}

func isStreaming(c *gin.Context) bool {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return req.Stream
}

func extractModel(c *gin.Context) string {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
		return ""
	}
	return req.Model
}

func extractTenantSlug(c *gin.Context) string {
	tCtx, ok := TenantFromContext(c)
	if ok {
		return tCtx.Slug().String()
	}
	slug := c.GetHeader("X-Tenant-ID")
	if slug == "" {
		return "unknown"
	}
	return slug
}

func updateMetrics(tenant, model string, inputTokens, outputTokens int64, cost float64) {
	metrics.TokensTotal.WithLabelValues(tenant, model, "input").Add(float64(inputTokens))
	metrics.TokensTotal.WithLabelValues(tenant, model, "output").Add(float64(outputTokens))
	metrics.CostTotal.WithLabelValues(tenant, model).Add(cost)
	metrics.RequestTotal.WithLabelValues(tenant, model).Inc()
}

type usageBodyWriter struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (w *usageBodyWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *usageBodyWriter) Write(b []byte) (int, error) {
	n, err := w.body.Write(b)
	if err != nil {
		return n, err
	}
	return w.ResponseWriter.Write(b)
}

func (w *usageBodyWriter) WriteString(s string) (int, error) {
	n, err := w.body.WriteString(s)
	if err != nil {
		return n, err
	}
	return w.ResponseWriter.WriteString(s)
}
