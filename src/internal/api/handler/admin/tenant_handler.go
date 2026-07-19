package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	dictionaryrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	domainErr "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task admin-ui-design#T3.2: AuditEvent for tenant CRUD audit logging (AC-005)
type AuditEvent struct {
	AdminUsername string          `json:"admin_username"`
	Action        string          `json:"action"`
	Target        string          `json:"target"`
	Details       json.RawMessage `json:"details,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// @sk-task admin-ui-design#T3.2: AuditLogger interface for async audit writes (AC-005)
type AuditLogger interface {
	Write(ctx context.Context, event *AuditEvent) error
}

// @sk-task admin-ui-design#T3.2: Extend TenantHandler with audit logger (AC-005)
type TenantHandler struct {
	repo            shield.TenantRepository
	dictionaryCache *dictionaryrepo.ValkeyDictionaryCache
	auditLog        AuditLogger
}

func NewTenantHandler(repo shield.TenantRepository, dictionaryCache *dictionaryrepo.ValkeyDictionaryCache, auditLog AuditLogger) *TenantHandler {
	return &TenantHandler{repo: repo, dictionaryCache: dictionaryCache, auditLog: auditLog}
}

func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var req dto.CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}

	slug, err := value.NewTenantSlug(req.Slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}

	if len(req.APIKeys) == 0 {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "api_keys must not be empty")
		return
	}

	authHeader := req.AuthHeader
	if authHeader == "" {
		authHeader = "X-Mask-Authorization"
	}

	dicts := make([]*dictionary.Dictionary, len(req.Dictionaries))
	for i, d := range req.Dictionaries {
		dicts[i] = dictionary.NewDictionary(d.Name, d.Entries, dictionary.MatchMode(d.MatchMode))
	}

	opts := []entity.TenantOption{entity.WithTenantDictionaries(dicts)}
	if req.PIIConfig != nil {
		opts = append(opts, entity.WithTenantPIIConfig(*req.PIIConfig))
	}
	tenant := entity.NewTenant(slug, req.Name, authHeader, req.APIKeys, opts...)

	if err := h.repo.Create(c.Request.Context(), tenant); err != nil {
		if errors.Is(err, domainErr.ErrDuplicateSlug) {
			middleware.AbortWithError(c, http.StatusConflict, middleware.ErrorCodeSlugConflict, "tenant slug already exists")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to create tenant")
		return
	}

	resp := dto.TenantToResponse(tenant)
	resp.Dictionaries = toDictionaryItems(dicts)
	c.JSON(http.StatusCreated, resp)

	if h.auditLog != nil {
		h.writeAudit(c, "create", "tenant:"+req.Slug, map[string]any{"slug": req.Slug, "name": req.Name})
	}
}

func (h *TenantHandler) ListTenants(c *gin.Context) {
	tenants, err := h.repo.List(c.Request.Context())
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to list tenants")
		return
	}

	resp := make([]dto.TenantResponse, len(tenants))
	for i, t := range tenants {
		resp[i] = dto.TenantToResponse(t)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *TenantHandler) GetTenant(c *gin.Context) {
	slugStr := c.Param("slug")
	slug, err := value.NewTenantSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid slug")
		return
	}

	tenant, err := h.repo.Get(c.Request.Context(), slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to get tenant")
		return
	}
	if tenant == nil {
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "tenant not found")
		return
	}

	resp := dto.TenantToResponse(tenant)
	resp.Dictionaries = toDictionaryItems(tenant.Dictionaries())
	c.JSON(http.StatusOK, resp)
}

func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	slugStr := c.Param("slug")
	slug, err := value.NewTenantSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid slug")
		return
	}

	var req dto.UpdateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}

	if len(req.APIKeys) == 0 {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "api_keys must not be empty")
		return
	}

	authHeader := req.AuthHeader
	if authHeader == "" {
		authHeader = "X-Mask-Authorization"
	}

	dicts := make([]*dictionary.Dictionary, len(req.Dictionaries))
	for i, d := range req.Dictionaries {
		dicts[i] = dictionary.NewDictionary(d.Name, d.Entries, dictionary.MatchMode(d.MatchMode))
	}

	opts := []entity.TenantOption{entity.WithTenantDictionaries(dicts)}
	if req.PIIConfig != nil {
		opts = append(opts, entity.WithTenantPIIConfig(*req.PIIConfig))
	}
	tenant := entity.NewTenant(slug, req.Name, authHeader, req.APIKeys, opts...)

	if err := h.repo.Update(c.Request.Context(), tenant); err != nil {
		if errors.Is(err, domainErr.ErrTenantNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "tenant not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to update tenant")
		return
	}

	resp := dto.TenantToResponse(tenant)
	resp.Dictionaries = toDictionaryItems(dicts)
	c.JSON(http.StatusOK, resp)

	if h.auditLog != nil {
		h.writeAudit(c, "update", "tenant:"+slugStr, map[string]any{"slug": slugStr, "name": req.Name})
	}
}

func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	slugStr := c.Param("slug")
	slug, err := value.NewTenantSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid slug")
		return
	}

	if err := h.repo.Delete(c.Request.Context(), slug); err != nil {
		if errors.Is(err, domainErr.ErrTenantNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "tenant not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to delete tenant")
		return
	}

	c.Status(http.StatusNoContent)

	if h.auditLog != nil {
		h.writeAudit(c, "delete", "tenant:"+slugStr, map[string]any{"slug": slugStr})
	}
}

func (h *TenantHandler) GetDictionaries(c *gin.Context) {
	slugStr := c.Param("slug")
	slug, err := value.NewTenantSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid slug")
		return
	}

	dicts, err := h.repo.GetDictionaries(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, domainErr.ErrTenantNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "tenant not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to get dictionaries")
		return
	}

	c.JSON(http.StatusOK, dto.DictionaryResponse{Dictionaries: toDictionaryItems(dicts)})
}

func (h *TenantHandler) UpdateDictionaries(c *gin.Context) {
	slugStr := c.Param("slug")
	slug, err := value.NewTenantSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid slug")
		return
	}

	var req dto.DictionaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}

	dicts := make([]*dictionary.Dictionary, len(req.Dictionaries))
	for i, d := range req.Dictionaries {
		dicts[i] = dictionary.NewDictionary(d.Name, d.Entries, dictionary.MatchMode(d.MatchMode))
	}

	if err := h.repo.UpdateDictionaries(c.Request.Context(), slug, dicts); err != nil {
		if errors.Is(err, domainErr.ErrTenantNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "tenant not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to update dictionaries")
		return
	}

	if h.dictionaryCache != nil {
		if err := h.dictionaryCache.Set(c.Request.Context(), slugStr, dicts); err != nil {
			c.Error(err) // non-fatal: logged but does not break the response
		}
	}

	c.JSON(http.StatusOK, dto.DictionaryResponse{Dictionaries: toDictionaryItems(dicts)})
}

func toDictionaryItems(dicts []*dictionary.Dictionary) []dto.DictionaryItem {
	if dicts == nil {
		return nil
	}
	items := make([]dto.DictionaryItem, len(dicts))
	for i, d := range dicts {
		items[i] = dto.DictionaryItem{
			Name:      d.Name(),
			MatchMode: d.MatchMode().String(),
			Entries:   d.Entries(),
		}
	}
	return items
}

// @sk-task admin-ui-design#T3.2: writeAudit sends an audit event async (AC-005)
func (h *TenantHandler) writeAudit(c *gin.Context, action, target string, details map[string]any) {
	detailsRaw, _ := json.Marshal(details)
	username, _ := c.Get("admin_username")
	usernameStr, _ := username.(string)

	h.auditLog.Write(c.Request.Context(), &AuditEvent{
		AdminUsername: usernameStr,
		Action:        action,
		Target:        target,
		Details:       detailsRaw,
		CreatedAt:     time.Now(),
	})
}
