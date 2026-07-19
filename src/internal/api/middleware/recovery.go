package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// @sk-task 10-gateway-skeleton#T2.3: Implement Recovery middleware with zap logging (AC-006)
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.ErrorContext(c.Request.Context(), "panic recovered",
					slog.Any("error", err),
					slog.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
