package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/compliance"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"

	complianceapp "github.com/bzdvdn/maskchain/src/internal/app/compliance"
)

// applyPackRequest is the payload for the apply-pack endpoint.
type applyPackRequest struct {
	PackKey string `json:"pack_key"`
}

// ComplianceHandler serves apply-pack and compliance-report admin endpoints.
type ComplianceHandler struct {
	registry *compliance.Registry
	repo     shield.TenantRepository
}

// NewComplianceHandler creates an admin handler over the pack registry and
// tenant repository.
func NewComplianceHandler(registry *compliance.Registry, repo shield.TenantRepository) *ComplianceHandler {
	return &ComplianceHandler{registry: registry, repo: repo}
}

// HandleApplyPack applies a compliance pack to a tenant in one call.
func (h *ComplianceHandler) HandleApplyPack(c *gin.Context) {
	slug, err := value.NewTenantSlug(c.Param("slug"))
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid tenant slug")
		return
	}
	var req applyPackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}
	if req.PackKey == "" {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "pack_key is required")
		return
	}

	svc := complianceapp.NewApplyPackService(h.registry, h.repo)
	res, err := svc.Apply(c.Request.Context(), slug, req.PackKey)
	if err != nil {
		h.writeApplyError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// HandleComplianceReport returns the active/deviated rules for a pack/tenant.
func (h *ComplianceHandler) HandleComplianceReport(c *gin.Context) {
	slug, err := value.NewTenantSlug(c.Param("slug"))
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid tenant slug")
		return
	}
	packKey := c.Query("pack")
	if packKey == "" {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "pack query parameter is required")
		return
	}

	svc := complianceapp.NewComplianceReportService(h.registry, h.repo)
	rep, err := svc.Report(c.Request.Context(), slug, packKey)
	if err != nil {
		h.writeApplyError(c, err)
		return
	}
	c.JSON(http.StatusOK, rep)
}

func (h *ComplianceHandler) writeApplyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, complianceapp.ErrPackNotFound):
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, err.Error())
	case errors.Is(err, complianceapp.ErrPackInvalid):
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
	case errors.Is(err, shielderrors.ErrTenantNotFound):
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "tenant not found")
	default:
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "compliance operation failed")
	}
}
