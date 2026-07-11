package incident

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 60-audit-incidents#T2.2: IncidentHandler scaffold (AC-001, AC-002)
type Handler struct {
	repo shield.IncidentRepository
}

func New(repo shield.IncidentRepository) *Handler {
	return &Handler{repo: repo}
}

// @sk-task 60-audit-incidents#T2.2: List incidents with filtering and pagination (AC-001, AC-006)
func (h *Handler) ListIncidents(c *gin.Context) {
	var params dto.IncidentFilterParams
	if err := c.ShouldBindQuery(&params); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid query parameters")
		return
	}

	filter := shield.IncidentFilter{
		Page:     params.Page,
		PageSize: params.PageSize,
	}
	if params.Severity != "" {
		s := params.Severity
		filter.Severity = &s
	}
	if params.Tenant != "" {
		t := params.Tenant
		filter.Tenant = &t
	}
	if params.ProfileSlug != "" {
		p := params.ProfileSlug
		filter.ProfileSlug = &p
	}

	incidents, total, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to list incidents")
		return
	}

	items := make([]dto.IncidentResponse, 0, len(incidents))
	for _, inc := range incidents {
		items = append(items, toResponse(inc))
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data:     items,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	})
}

// @sk-task 60-audit-incidents#T2.2: Get single incident by ID (AC-002, AC-007)
func (h *Handler) GetIncident(c *gin.Context) {
	id := c.Param("id")

	inc, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to find incident")
		return
	}
	if inc == nil {
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "incident not found")
		return
	}

	c.JSON(http.StatusOK, toResponse(inc))
}

func toResponse(inc *entity.Incident) dto.IncidentResponse {
	resp := dto.IncidentResponse{
		ID:                    inc.Slug(),
		RequestID:             inc.RequestID(),
		Timestamp:             inc.Timestamp().Format(time.RFC3339),
		Tenant:                inc.Tenant(),
		ProfileSlug:           inc.ProfileSlug(),
		DetectorType:          inc.DetectorType(),
		EntryValue:            inc.EntryValue(),
		Severity:              inc.Severity().String(),
		Action:                inc.Action(),
		PromptSnippetRedacted: inc.PromptSnippetRedacted(),
		ResponseSnippet:       inc.ResponseSnippet(),
	}
	return resp
}
