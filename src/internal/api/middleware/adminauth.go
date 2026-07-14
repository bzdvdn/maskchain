package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 90-production-hardening#T2.1: AdminAuth middleware for pprof access (<AC-001>)
func AdminAuth(cfg *config.DebugConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil || !cfg.Enabled {
			c.Next()
			return
		}
		token := c.GetHeader("X-Admin-Token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AdminToken)) != 1 {
			AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
			return
		}
		c.Next()
	}
}
