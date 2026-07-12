package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @sk-task 10-gateway-skeleton#T3.1: Implement Logger middleware with zap (AC-007)
func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		rid, _ := c.Get(requestIDKey)
		ridStr, _ := rid.(string)

		// @sk-task 80-tenant-isolation#T3.2: Add tenant_id to log attributes (AC-008)
		logFields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", duration),
			zap.String("request_id", ridStr),
		}
		if tid, ok := TenantFromContext(c); ok {
			logFields = append(logFields, zap.String("tenant_id", tid))
		}
		log.Info("request", logFields...)
	}
}
