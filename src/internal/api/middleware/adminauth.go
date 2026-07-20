package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

type adminContextKey struct{}

var AdminSessionKey adminContextKey

func AdminFromContext(ctx context.Context) (*admin_session.AdminSession, bool) {
	s, ok := ctx.Value(AdminSessionKey).(*admin_session.AdminSession)
	return s, ok
}

// @sk-task 90-production-hardening#T2.1: AdminAuth middleware for pprof access (<AC-001>)
//
// AdminAuth handles the operation.
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

// @sk-task admin-ui-design#T2.1: AdminSessionAuth middleware validates admin session cookie/header (AC-001, AC-004)
//
// AdminSessionAuth handles the operation.
func AdminSessionAuth(useCase *admin_session.AdminSessionUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			cookie, err := c.Cookie("admin_token")
			if err != nil {
				AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
				return
			}
			token = "Bearer " + cookie
		}

		if !strings.HasPrefix(token, "Bearer ") {
			AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
			return
		}
		rawToken := strings.TrimPrefix(token, "Bearer ")

		sess, err := useCase.Validate(c.Request.Context(), rawToken)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
			return
		}

		c.Set("admin_username", sess.Username)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), AdminSessionKey, sess))
		c.Next()
	}
}

// @sk-task seed-tenant-fix#T1.1: Combined middleware tries Bearer session first, falls back to X-Admin-Token (AC-001)
// @sk-task seed-tenant-fix#T1.2: Added checkTenantKey fallback for tenant API key auth (AC-001)
//
// AdminSessionOrTokenAuth handles the operation.
func AdminSessionOrTokenAuth(useCase *admin_session.AdminSessionUseCase, cfg *config.DebugConfig, checkTenantKey func(ctx context.Context, apiKey string) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken := ""

		token := c.GetHeader("Authorization")
		if token == "" {
			cookie, err := c.Cookie("admin_token")
			if err == nil {
				token = "Bearer " + cookie
			}
		}

		if strings.HasPrefix(token, "Bearer ") {
			rawToken = strings.TrimPrefix(token, "Bearer ")
			sess, err := useCase.Validate(c.Request.Context(), rawToken)
			if err == nil {
				c.Set("admin_username", sess.Username)
				c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), AdminSessionKey, sess))
				c.Next()
				return
			}
		}

		if cfg != nil && cfg.Enabled {
			adminToken := c.GetHeader("X-Admin-Token")
			if adminToken != "" && subtle.ConstantTimeCompare([]byte(adminToken), []byte(cfg.AdminToken)) == 1 {
				c.Next()
				return
			}
		}

		if checkTenantKey != nil && rawToken != "" && checkTenantKey(c.Request.Context(), rawToken) {
			c.Next()
			return
		}

		AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
	}
}
