package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task admin-ui-design#T2.2: AdminAuthHandler with login/logout (AC-001)
//
// AdminAuthHandler represents a domain entity or configuration.
type AdminAuthHandler struct {
	useCase *admin_session.AdminSessionUseCase
	cfg     *config.AdminConfig
}

func NewAdminAuthHandler(useCase *admin_session.AdminSessionUseCase, cfg *config.AdminConfig) *AdminAuthHandler {
	return &AdminAuthHandler{useCase: useCase, cfg: cfg}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

func (h *AdminAuthHandler) HandleLogin(c *gin.Context) {
	if h.cfg == nil || h.cfg.Username == "" || h.cfg.Password == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "admin not configured"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Username != h.cfg.Username || req.Password != h.cfg.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	ttl := h.cfg.SessionTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}

	_, rawToken, err := h.useCase.Create(c.Request.Context(), req.Username, ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	expiresAt := time.Now().Add(ttl)
	c.SetCookie("admin_token", rawToken, int(ttl.Seconds()), "/", "", false, true)
	c.JSON(http.StatusOK, loginResponse{
		Token:     rawToken,
		ExpiresAt: expiresAt.Unix(),
	})
}

func (h *AdminAuthHandler) HandleLogout(c *gin.Context) {
	token, err := c.Cookie("admin_token")
	if err == nil && token != "" {
		sess, ok := middleware.AdminFromContext(c.Request.Context())
		if ok {
			h.useCase.Delete(c.Request.Context(), sess.ID)
		}
		c.SetCookie("admin_token", "", -1, "/", "", false, true)
	}
	c.JSON(http.StatusOK, gin.H{"status": "logged out"})
}

// @sk-task admin-ui-design#T4.2: HandleVerify returns current session info (AC-001, AC-004)
func (h *AdminAuthHandler) HandleVerify(c *gin.Context) {
	sess, ok := middleware.AdminFromContext(c.Request.Context())
	if !ok || sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"username":   sess.Username,
		"expires_at": sess.ExpiresAt.Unix(),
	})
}
