package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	domainErr "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task tenant-profile-sync#T2.2: TenantHandler for admin CRUD (AC-001, AC-005, AC-008)
type TenantHandler struct {
	repo shield.TenantRepository
}

func NewTenantHandler(repo shield.TenantRepository) *TenantHandler {
	return &TenantHandler{repo: repo}
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
