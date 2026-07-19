package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// @sk-task admin-ui-design#T3.2: AuditLogReader interface for listing audit events (AC-005)
type AuditLogReader interface {
	List(ctx context.Context, limit, offset int) ([]AuditEvent, error)
}

// @sk-task admin-ui-design#T3.2: AuditHandler for GET /api/v1/audit (AC-005)
type AuditHandler struct {
	store AuditLogReader
}

func NewAuditHandler(store AuditLogReader) *AuditHandler {
	return &AuditHandler{store: store}
}

type auditEntryResponse struct {
	ID            int64     `json:"id"`
	AdminUsername string    `json:"admin_username"`
	Action        string    `json:"action"`
	Target        string    `json:"target"`
	Details       any       `json:"details,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type auditListResponse struct {
	Items  []auditEntryResponse `json:"items"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

func (h *AuditHandler) HandleList(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	entries, err := h.store.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit log"})
		return
	}

	items := make([]auditEntryResponse, len(entries))
	for i, e := range entries {
		var details any
		if e.Details != nil {
			details = string(e.Details)
		}
		items[i] = auditEntryResponse{
			ID:            int64(i + 1),
			AdminUsername: e.AdminUsername,
			Action:        e.Action,
			Target:        e.Target,
			Details:       details,
			CreatedAt:     e.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, auditListResponse{
		Items:  items,
		Total:  len(items),
		Limit:  limit,
		Offset: offset,
	})
}
