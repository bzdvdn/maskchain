package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"

)

type createSessionRequest struct {
	Model string `json:"model" binding:"required"`
}

type extendTTLRequest struct {
	TTLSeconds int32 `json:"ttl_seconds" binding:"required"`
}

// @sk-task sessions#T2.2: Implement SessionHandler with all routes (AC-001, AC-003, AC-004, AC-005, AC-006)
type SessionHandler struct {
	useCase *session.SessionUseCase
	cfg     *config.SessionConfig
}

func NewSessionHandler(useCase *session.SessionUseCase, cfg *config.SessionConfig) *SessionHandler {
	return &SessionHandler{useCase: useCase, cfg: cfg}
}

// @sk-task sessions#T2.2: Implement HandleCreate (AC-001)
func (h *SessionHandler) HandleCreate(c *gin.Context) {
	tenant, ok := middleware.TenantFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant"})
		return
	}
	tenantID := tenant.Slug().String()

	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %s", err)})
		return
	}

	ttl := h.cfg.DefaultTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}

	sess, err := h.useCase.Create(c.Request.Context(), session.NewSessionID(), tenantID, req.Model, ttl)
	if err != nil {
		if errors.Is(err, session.ErrSessionConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "session ID already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, sessionToResponse(sess))
}

// @sk-task sessions#T2.2: Implement HandleGet (AC-003)
func (h *SessionHandler) HandleGet(c *gin.Context) {
	tenant, ok := middleware.TenantFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant"})
		return
	}
	tenantID := tenant.Slug().String()

	sessionID := c.Param("id")
	sess, err := h.useCase.Get(c.Request.Context(), tenantID, sessionID)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if sess.Status == session.SessionStatusExpired {
		c.JSON(http.StatusGone, gin.H{"error": "session is expired"})
		return
	}

	c.JSON(http.StatusOK, sessionToResponse(sess))
}

// @sk-task sessions#T2.2: Implement HandleList (AC-004)
func (h *SessionHandler) HandleList(c *gin.Context) {
	tenant, ok := middleware.TenantFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant"})
		return
	}
	tenantID := tenant.Slug().String()

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	result, err := h.useCase.ListByTenant(c.Request.Context(), tenantID, int32(page), int32(limit))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]gin.H, len(result.Items))
	for i, s := range result.Items {
		items[i] = sessionToResponse(&s)
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": result.Total,
		"page":  result.Page,
		"limit": result.Limit,
	})
}

// @sk-task sessions#T2.2: Implement HandleExtend (AC-005)
func (h *SessionHandler) HandleExtend(c *gin.Context) {
	tenant, ok := middleware.TenantFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant"})
		return
	}
	tenantID := tenant.Slug().String()

	sessionID := c.Param("id")
	var req extendTTLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %s", err)})
		return
	}

	if req.TTLSeconds <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ttl_seconds must be positive"})
		return
	}

	ttl := time.Duration(req.TTLSeconds) * time.Second
	if h.cfg.MaxTTL > 0 && ttl > h.cfg.MaxTTL {
		ttl = h.cfg.MaxTTL
	}

	sess, err := h.useCase.ExtendTTL(c.Request.Context(), tenantID, sessionID, int32(ttl.Seconds()))
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		if errors.Is(err, session.ErrSessionExpired) {
			c.JSON(http.StatusGone, gin.H{"error": "session is expired"})
			return
		}
		if errors.Is(err, session.ErrSessionClosed) {
			c.JSON(http.StatusConflict, gin.H{"error": "session is closed"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sessionToResponse(sess))
}

// @sk-task sessions#T2.2: Implement HandleClose (AC-006)
func (h *SessionHandler) HandleClose(c *gin.Context) {
	tenant, ok := middleware.TenantFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant"})
		return
	}
	tenantID := tenant.Slug().String()

	sessionID := c.Param("id")
	if err := h.useCase.Close(c.Request.Context(), tenantID, sessionID); err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		if errors.Is(err, session.ErrSessionExpired) {
			c.JSON(http.StatusGone, gin.H{"error": "session is expired"})
			return
		}
		if errors.Is(err, session.ErrSessionClosed) {
			c.JSON(http.StatusConflict, gin.H{"error": "session is already closed"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "closed"})
}

func sessionToResponse(s *session.Session) gin.H {
	return gin.H{
		"session_id":         s.SessionID,
		"tenant_id":          s.TenantID,
		"model":              s.Model,
		"token_count":        s.TokenCount,
		"message_count":      s.MessageCount,
		"total_masks":        s.TotalMasks,
		"dict_mask_count":    s.DictMaskCount,
		"pii_mask_count":     s.PIIMaskCount,
		"preprocessor_count": s.PreprocessorCount,
		"status":             s.Status,
		"created_at":         s.CreatedAt,
		"expires_at":         s.ExpiresAt,
	}
}
