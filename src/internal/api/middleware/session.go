package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

const sessionCtxKey = "session"

// @sk-task sessions#T4.1: SessionFromContext reads Session from gin context (AC-010)
//
// SessionFromContext handles the operation.
func SessionFromContext(c *gin.Context) (*session.Session, bool) {
	v, ok := c.Get(sessionCtxKey)
	if !ok {
		return nil, false
	}
	s, ok := v.(*session.Session)
	return s, ok
}

type sessionChatBody struct {
	Model string `json:"model"`
}

// @sk-task sessions#T4.1: Implement SessionMiddleware (AC-010)
//
// SessionMiddleware handles the operation.
func SessionMiddleware(useCase *session.SessionUseCase, cfg *config.SessionConfig, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.GetHeader("X-Session-ID")
		if sessionID == "" {
			c.Next()
			return
		}

		tenant, ok := TenantFromContext(c)
		if !ok {
			c.Next()
			return
		}
		tenantID := tenant.Slug().String()

		existing, err := useCase.Get(c.Request.Context(), tenantID, sessionID)
		if err == nil && existing != nil {
			c.Set(sessionCtxKey, existing)
			c.Header("X-Session-ID", sessionID)
			c.Next()
			return
		}

		body, err := c.GetRawData()
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var chatBody sessionChatBody
		if err := json.Unmarshal(body, &chatBody); err != nil || chatBody.Model == "" {
			c.Next()
			return
		}

		ttl := cfg.DefaultTTL
		if ttl <= 0 {
			ttl = 30 * time.Minute
		}

		sess, err := useCase.Create(c.Request.Context(), sessionID, tenantID, chatBody.Model, ttl)
		if err != nil {
			c.Next()
			return
		}

		c.Set(sessionCtxKey, sess)
		c.Header("X-Session-ID", sessionID)
		c.Next()
	}
}
