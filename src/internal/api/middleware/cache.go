package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	cacheapp "github.com/bzdvdn/maskchain/src/internal/app/cache"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// @sk-task semantic-cache-masked#T2.2: SemanticCacheMiddleware (AC-001, AC-002, AC-007)
//
// SemanticCacheMiddleware hooks after the shield middleware. It embeds the
// already-masked prompt, serves a cache hit from Valkey without calling the
// provider, and stores masked responses on a miss. Any failure degrades to
// transparent passthrough. Streaming requests and non-chat paths are skipped.
type SemanticCacheMiddleware struct {
	service *cacheapp.SemanticCacheService
	cfg     *config.CacheConfig
	log     *slog.Logger
}

func NewSemanticCacheMiddleware(service *cacheapp.SemanticCacheService, cfg *config.CacheConfig, log *slog.Logger) *SemanticCacheMiddleware {
	return &SemanticCacheMiddleware{service: service, cfg: cfg, log: log}
}

// @sk-task semantic-cache-masked#T2.2: Handler applies the cache (AC-007)
func (m *SemanticCacheMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		if m.cfg == nil || !m.cfg.Enabled || m.service == nil {
			c.Next()
			return
		}

		tenant, ok := TenantFromContext(c)
		if !ok {
			c.Next()
			return
		}

		body, err := c.GetRawData()
		if err != nil || len(body) == 0 {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		var chatReq chatRequest
		if err := json.Unmarshal(body, &chatReq); err != nil {
			c.Next()
			return
		}
		if chatReq.Stream {
			c.Next()
			return
		}
		maskedText := extractPromptText(chatReq.Messages)
		if maskedText == "" {
			c.Next()
			return
		}

		tenantSlug := tenant.Slug().String()
		ctx := cacheapp.WithTenant(c.Request.Context(), tenantSlug)

		payload, err := m.service.Lookup(ctx, tenantSlug, maskedText)
		if err != nil {
			metrics.CacheErrorsTotal.WithLabelValues(tenantSlug).Inc()
			m.log.WarnContext(c.Request.Context(), "semantic cache lookup failed, passing through",
				slog.String("error", err.Error()), slog.String("tenant", tenantSlug))
			c.Next()
			return
		}
		if payload != nil {
			metrics.CacheHitsTotal.WithLabelValues(tenantSlug).Inc()
			w := &usageBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
			c.Writer = w
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
			c.Abort()
			return
		}

		// Miss: run the provider path and cache the masked response.
		metrics.CacheMissesTotal.WithLabelValues(tenantSlug).Inc()
		metrics.CacheKeysTotal.WithLabelValues(tenantSlug).Inc()
		w := &usageBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = w
		c.Next()
		if w.status != http.StatusOK {
			return
		}
		blocked, err := m.service.Store(ctx, tenantSlug, maskedText, w.body.Bytes())
		if blocked {
			metrics.CacheWriteBlockedTotal.WithLabelValues(tenantSlug).Inc()
			return
		}
		if err != nil {
			metrics.CacheErrorsTotal.WithLabelValues(tenantSlug).Inc()
			m.log.WarnContext(c.Request.Context(), "semantic cache store failed",
				slog.String("error", err.Error()), slog.String("tenant", tenantSlug))
			return
		}
	}
}
