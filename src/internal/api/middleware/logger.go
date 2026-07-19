package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// @sk-task 10-gateway-skeleton#T3.1: Implement Logger middleware with zap (AC-007)
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		rid, _ := c.Get(requestIDKey)
		ridStr, _ := rid.(string)

		// @sk-task 80-tenant-isolation#T3.2: Add tenant_id to log attributes (AC-008)
		args := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("duration", duration),
			slog.String("request_id", ridStr),
		}
		if t, ok := TenantFromContext(c); ok {
			args = append(args, slog.String("tenant_id", t.Slug().String()))
		}
		log.InfoContext(c.Request.Context(), "request", args...)
	}
}
