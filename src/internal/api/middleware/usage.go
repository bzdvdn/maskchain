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
//
// UsageMiddleware represents a domain entity or configuration.
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

		// @sk-task usage-accounting-integrity#T2.2: record analytics for streamed and non-streamed responses (AC-003, AC-006, AC-007)
		tenantStr := extractTenantSlug(c)
		streaming := isStreaming(c)
		capture := wrapUsageCapture(c, streaming)
		c.Next()

		if capture.status != http.StatusOK {
			return
		}

		usage, ok := capture.Usage()
		if !ok {
			metrics.UsageMissingTotal.WithLabelValues(tenantStr, model).Inc()
			m.log.WarnContext(c.Request.Context(), "usage middleware: provider reported no usage; request not recorded",
				slog.String("path", c.Request.URL.Path), slog.String("model", model), slog.Bool("streaming", streaming))
			return
		}

		rate, source := m.registry.Resolve(model)
		switch source {
		case analytics.CostRateSourceFallback:
			metrics.CostRateFallbackTotal.WithLabelValues(model).Inc()
		case analytics.CostRateSourceMissing:
			metrics.CostRateMissingTotal.WithLabelValues(model).Inc()
		}

		tenantID, _ := value.NewTenantID(tenantStr)
		cost := rate.Cost(usage.PromptTokens, usage.CompletionTokens)

		record := analytics.TokenUsage{
			TenantID:     tenantID,
			Model:        model,
			InputTokens:  usage.PromptTokens,
			OutputTokens: usage.CompletionTokens,
			Cost:         cost,
			Timestamp:    time.Now(),
		}

		updateMetrics(tenantStr, model, usage.PromptTokens, usage.CompletionTokens, cost)

		select {
		case m.usageCh <- record:
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
