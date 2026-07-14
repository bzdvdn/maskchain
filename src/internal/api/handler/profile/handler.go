package profile

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// @sk-task 40-profiles-api#T1.2: Create ProfileHandler scaffold (AC-001)
type ProfileHandler struct {
	repo shield.ProfileRepository
}

func New(repo shield.ProfileRepository) *ProfileHandler {
	return &ProfileHandler{repo: repo}
}

// @sk-task 80-tenant-isolation#T2.3: Read tenant from auth middleware context, abort if missing (AC-005)
func tenantIDFromContext(c *gin.Context) value.TenantID {
	slug, ok := middleware.TenantFromContext(c)
	if !ok {
		middleware.AbortWithError(c, http.StatusUnauthorized, middleware.ErrorCodeUnauthorized, "unauthorized")
		return value.TenantID{}
	}
	tid, err := value.NewTenantID(slug.Slug().String())
	if err != nil {
		middleware.AbortWithError(c, http.StatusUnauthorized, middleware.ErrorCodeUnauthorized, "unauthorized")
		return value.TenantID{}
	}
	return tid
}

// @sk-task 40-profiles-api#T2.1: Implement CreateProfile handler (AC-001, AC-002, AC-003)
func (h *ProfileHandler) CreateProfile(c *gin.Context) {
	var req dto.CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c, err)
		return
	}

	if err := validate.Struct(req); err != nil {
		writeValidationError(c, err)
		return
	}

	slug, err := value.NewProfileSlug(req.Slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}

	tenantID := tenantIDFromContext(c)

	existing, err := h.repo.FindBySlug(c.Request.Context(), tenantID, slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to check slug uniqueness")
		return
	}
	if existing != nil {
		middleware.AbortWithError(c, http.StatusConflict, middleware.ErrorCodeSlugConflict, "slug already exists")
		return
	}

	pid, err := value.NewProfileID(uuid.New().String())
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to generate profile id")
		return
	}

	dicts := makeDictionaries(slug, req.Dictionaries)

	var pps []preprocessor.PreprocessorDef
	if req.Preprocessors != nil {
		pps = req.Preprocessors
	}

	profile := entity.NewProfile(pid, slug, tenantID, req.Name,
		entity.WithDictionaries(dicts),
		entity.WithPreprocessors(pps),
	)
	if req.Description != nil {
		entity.WithDescription(*req.Description)(profile)
	}

	if err := h.repo.Save(c.Request.Context(), profile); err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, fmt.Sprintf("failed to save profile: %s", err))
		return
	}

	c.JSON(http.StatusCreated, toProfileResponse(profile))
}

// @sk-task 40-profiles-api#T2.2: Implement ListProfiles and GetProfile handlers (AC-004, AC-005, AC-006)
// @sk-task 41-profiles-ui#T2.1: Add pagination to ListProfiles (AC-002)
// @sk-task 118-api-consistency#T3.1: Updated to per_page pagination via ApiResponse (AC-005)
func (h *ProfileHandler) ListProfiles(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPageStr := c.DefaultQuery("per_page", "")
	perPage := 0
	if perPageStr == "" {
		perPage, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	} else {
		perPage, _ = strconv.Atoi(perPageStr)
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	profiles, err := h.repo.ListByTenant(c.Request.Context(), tenantID)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to list profiles")
		return
	}

	total := len(profiles)
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	items := make([]dto.ProfileListItem, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, toProfileListItem(p))
	}

	c.Set("pagination", dto.Pagination{Page: page, PerPage: perPage, Total: total})
	c.JSON(http.StatusOK, items[start:end])
}

// @sk-task 40-profiles-api#T2.2: Implement ListProfiles and GetProfile handlers (AC-004, AC-005, AC-006)
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	slugStr := c.Param("slug")

	slug, err := value.NewProfileSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, fmt.Sprintf("invalid slug: %s", err))
		return
	}

	tenantID := tenantIDFromContext(c)

	profile, err := h.repo.FindBySlug(c.Request.Context(), tenantID, slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to find profile")
		return
	}
	if profile == nil {
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "profile not found")
		return
	}

	c.JSON(http.StatusOK, toProfileResponse(profile))
}

// @sk-task 40-profiles-api#T3.1: Implement UpdateProfile and DeleteProfile handlers (AC-007, AC-008)
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	slugStr := c.Param("slug")

	slug, err := value.NewProfileSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, fmt.Sprintf("invalid slug: %s", err))
		return
	}

	tenantID := tenantIDFromContext(c)

	existing, err := h.repo.FindBySlug(c.Request.Context(), tenantID, slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to find profile")
		return
	}
	if existing == nil {
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "profile not found")
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c, err)
		return
	}

	if err := validate.Struct(req); err != nil {
		writeValidationError(c, err)
		return
	}

	name := existing.Name()
	if req.Name != nil {
		name = *req.Name
	}

	description := existing.Description()
	if req.Description != nil {
		description = req.Description
	}

	dicts := existing.Dictionaries()
	if req.Dictionaries != nil {
		dicts = makeDictionaries(slug, req.Dictionaries)
	}

	pps := existing.Preprocessors()
	if req.Preprocessors != nil {
		pps = req.Preprocessors
	}

	profile := entity.NewProfile(existing.ID(), slug, tenantID, name,
		entity.WithDictionaries(dicts),
		entity.WithPreprocessors(pps),
		entity.WithEnabled(existing.Enabled()),
	)
	if description != nil {
		entity.WithDescription(*description)(profile)
	}

	if err := h.repo.Save(c.Request.Context(), profile); err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, fmt.Sprintf("failed to update profile: %s", err))
		return
	}

	c.JSON(http.StatusOK, toProfileResponse(profile))
}

// @sk-task 40-profiles-api#T3.1: Implement UpdateProfile and DeleteProfile handlers (AC-007, AC-008)
func (h *ProfileHandler) DeleteProfile(c *gin.Context) {
	slugStr := c.Param("slug")

	slug, err := value.NewProfileSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, fmt.Sprintf("invalid slug: %s", err))
		return
	}

	tenantID := tenantIDFromContext(c)

	existing, err := h.repo.FindBySlug(c.Request.Context(), tenantID, slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to find profile")
		return
	}
	if existing == nil {
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "profile not found")
		return
	}

	if err := h.repo.Delete(c.Request.Context(), existing.ID()); err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, fmt.Sprintf("failed to delete profile: %s", err))
		return
	}

	c.Status(http.StatusNoContent)
}

// @sk-task 40-profiles-api#T3.2: Implement PatchDictionary handler (AC-009, AC-010)
func (h *ProfileHandler) PatchDictionary(c *gin.Context) {
	slugStr := c.Param("slug")

	slug, err := value.NewProfileSlug(slugStr)
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, fmt.Sprintf("invalid slug: %s", err))
		return
	}

	tenantID := tenantIDFromContext(c)

	var req dto.PatchDictionaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c, err)
		return
	}

	if err := validate.Struct(req); err != nil {
		writeValidationError(c, err)
		return
	}

	profile, err := h.repo.FindBySlug(c.Request.Context(), tenantID, slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to find profile")
		return
	}
	if profile == nil {
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "profile not found")
		return
	}

	dicts := profile.Dictionaries()
	var targetDict *dictionary.Dictionary
	for _, d := range dicts {
		if d.Name() == req.Name {
			targetDict = d
			break
		}
	}

	if targetDict == nil {
		if req.Action == "remove" {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, fmt.Sprintf("dictionary %q not found", req.Name))
			return
		}

		entryList := make([]interface{}, len(req.Entries))
		for i, e := range req.Entries {
			entryList[i] = e
		}
		targetDict = dictionary.NewDictionary(slug, req.Name, entryList, dictionary.MatchModeExact)
		dicts = append(dicts, targetDict)

		profile = entity.NewProfile(profile.ID(), slug, tenantID, profile.Name(),
			entity.WithDictionaries(dicts),
			entity.WithPreprocessors(profile.Preprocessors()),
			entity.WithEnabled(profile.Enabled()),
		)
		if profile.Description() != nil {
			entity.WithDescription(*profile.Description())(profile)
		}

		if err := h.repo.Save(c.Request.Context(), profile); err != nil {
			middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, fmt.Sprintf("failed to patch dictionary: %s", err))
			return
		}

		c.JSON(http.StatusOK, toProfileResponse(profile))
		return
	}

	existingRaw := targetDict.Entries()
	switch req.Action {
	case "add":
		for _, e := range req.Entries {
			existingRaw = append(existingRaw, e)
		}
	case "remove":
		removeSet := make(map[string]struct{}, len(req.Entries))
		for _, e := range req.Entries {
			removeSet[e] = struct{}{}
		}
		var filtered []interface{}
		for _, e := range existingRaw {
			if s, ok := e.(string); ok {
				if _, ok := removeSet[s]; !ok {
					filtered = append(filtered, s)
				}
			} else {
				filtered = append(filtered, e)
			}
		}
		existingRaw = filtered
	}

	updatedDict := dictionary.NewDictionary(slug, req.Name, existingRaw, targetDict.MatchMode())
	newDicts := make([]*dictionary.Dictionary, 0, len(dicts))
	for _, d := range dicts {
		if d.Name() == req.Name {
			newDicts = append(newDicts, updatedDict)
		} else {
			newDicts = append(newDicts, d)
		}
	}

	profile = entity.NewProfile(profile.ID(), slug, tenantID, profile.Name(),
		entity.WithDictionaries(newDicts),
		entity.WithPreprocessors(profile.Preprocessors()),
		entity.WithEnabled(profile.Enabled()),
	)
	if profile.Description() != nil {
		entity.WithDescription(*profile.Description())(profile)
	}

	if err := h.repo.Save(c.Request.Context(), profile); err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, fmt.Sprintf("failed to patch dictionary: %s", err))
		return
	}

	c.JSON(http.StatusOK, toProfileResponse(profile))
}

func makeDictionaries(slug value.ProfileSlug, dictDTOs []dto.DictionaryDTO) []*dictionary.Dictionary {
	if len(dictDTOs) == 0 {
		return nil
	}
	dicts := make([]*dictionary.Dictionary, 0, len(dictDTOs))
	for _, d := range dictDTOs {
		dicts = append(dicts, dictionary.NewDictionary(slug, d.Name, d.Entries, d.MatchMode))
	}
	return dicts
}

// makeStringEntries adapts []interface{} entries to []string for PatchDictionary.
func makeStringEntries(raw []interface{}) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func toProfileResponse(p *entity.Profile) dto.ProfileResponse {
	resp := dto.ProfileResponse{
		ID:            p.ID().String(),
		Slug:          p.Slug().String(),
		Name:          p.Name(),
		Description:   p.Description(),
		Preprocessors: p.Preprocessors(),
		CreatedAt:     p.CreatedAt().Format(time.RFC3339),
		UpdatedAt:     p.UpdatedAt().Format(time.RFC3339),
	}

	if p.Enabled() {
		resp.Status = "active"
	} else {
		resp.Status = "disabled"
	}

	dictDTOs := make([]dto.DictionaryDTO, 0, len(p.Dictionaries()))
	for _, d := range p.Dictionaries() {
		dictDTOs = append(dictDTOs, dto.DictionaryDTO{
			Name:      d.Name(),
			Entries:   d.Entries(),
			MatchMode: d.MatchMode(),
		})
	}
	resp.Dictionaries = dictDTOs

	return resp
}

func toProfileListItem(p *entity.Profile) dto.ProfileListItem {
	item := dto.ProfileListItem{
		Slug: p.Slug().String(),
		Name: p.Name(),
	}
	if p.Enabled() {
		item.Status = "active"
	} else {
		item.Status = "disabled"
	}
	return item
}

func writeValidationError(c *gin.Context, err error) {
	var verr validator.ValidationErrors
	if errors.As(err, &verr) {
		details := make([]dto.ValidationDetail, 0, len(verr))
		for _, fe := range verr {
			details = append(details, dto.ValidationDetail{
				Field:   fe.Field(),
				Message: fe.Tag(),
			})
		}
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "validation failed", details...)
		return
	}

	middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
}
