package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// @sk-task 300-virtual-keys#T3.1: VirtualKeyHandler manages virtual keys via admin API (AC-001)
// @sk-task 403-key-at-rest-encryption#T3.3: VirtualKeyHandler invalidates the auth cache on mutations (AC-005)
type VirtualKeyHandler struct {
	repo     virtualkey.VirtualKeyRepository
	auditLog AuditLogger
	cache    *middleware.VirtualKeyCache
}

// NewVirtualKeyHandler builds a handler. A non-nil cache is invalidated after
// create/update/delete so gateway auth does not serve stale keys (revoked keys
// stop authenticating immediately in-process).
func NewVirtualKeyHandler(repo virtualkey.VirtualKeyRepository, auditLog AuditLogger, cache ...*middleware.VirtualKeyCache) *VirtualKeyHandler {
	var vkCache *middleware.VirtualKeyCache
	if len(cache) > 0 {
		vkCache = cache[0]
	}
	return &VirtualKeyHandler{repo: repo, auditLog: auditLog, cache: vkCache}
}

// @sk-task 300-virtual-keys#T3.1: Create generates a key and returns plaintext once (AC-001)
func (h *VirtualKeyHandler) Create(c *gin.Context) {
	var req dto.CreateVirtualKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}
	if req.TenantID == "" {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "tenant_id is required")
		return
	}

	id, err := virtualkey.NewKeyID()
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to generate key")
		return
	}

	raw, err := virtualkey.NewSecret()
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to generate secret")
		return
	}

	now := time.Now().UTC()
	key := &virtualkey.VirtualKey{
		ID:            id,
		TenantID:      req.TenantID,
		KeyHash:       virtualkey.KeyHash(raw),
		Label:         req.Label,
		AllowedModels: req.AllowedModels,
		BlockedModels: req.BlockedModels,
		BudgetCap:     req.BudgetCap,
		ExpiresAt:     req.ExpiresAt,
		Metadata:      req.Metadata,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if key.Metadata == nil {
		key.Metadata = map[string]string{}
	}

	if err := h.repo.Create(c.Request.Context(), key); err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to create key")
		return
	}
	h.invalidateKey(key.ID)

	h.writeKeyAudit(c, "create_key", key.ID, map[string]any{"tenant_id": req.TenantID, "label": req.Label})

	resp := dto.CreateVirtualKeyResponse{
		VirtualKeyResponse: dto.VirtualKeyToResponse(key),
		Key:                raw,
	}
	c.JSON(http.StatusCreated, resp)
}

// @sk-task 300-virtual-keys#T3.1: List returns all keys (AC-001)
func (h *VirtualKeyHandler) List(c *gin.Context) {
	q := parseListQuery(c)

	var (
		keys  []*virtualkey.VirtualKey
		total int
		err   error
	)
	if q.Active {
		keys, total, err = h.repo.ListPaged(c.Request.Context(), q.Limit, q.Offset, q.Search)
	} else {
		keys, err = h.repo.List(c.Request.Context())
		total = len(keys)
	}
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to list keys")
		return
	}
	out := make([]dto.VirtualKeyResponse, len(keys))
	for i, k := range keys {
		out[i] = dto.VirtualKeyToResponse(k)
	}
	if q.Active {
		writePage(c, out, q, total)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// @sk-task 300-virtual-keys#T3.1: ListByTenant returns keys of a tenant (AC-001)
func (h *VirtualKeyHandler) ListByTenant(c *gin.Context) {
	tenantID := c.Param("slug")
	keys, err := h.repo.ListByTenant(c.Request.Context(), tenantID)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to list keys")
		return
	}
	out := make([]dto.VirtualKeyResponse, len(keys))
	for i, k := range keys {
		out[i] = dto.VirtualKeyToResponse(k)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// @sk-task 300-virtual-keys#T3.1: Get returns a single key (AC-001)
func (h *VirtualKeyHandler) Get(c *gin.Context) {
	key, err := h.repo.GetById(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, virtualkey.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "key not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to get key")
		return
	}
	c.JSON(http.StatusOK, dto.VirtualKeyToResponse(key))
}

// @sk-task 300-virtual-keys#T3.1: Update modifies key scopes/state (AC-001)
func (h *VirtualKeyHandler) Update(c *gin.Context) {
	var req dto.UpdateVirtualKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}

	key, err := h.repo.GetById(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, virtualkey.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "key not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to get key")
		return
	}

	key.Label = req.Label
	key.AllowedModels = req.AllowedModels
	key.BlockedModels = req.BlockedModels
	key.BudgetCap = req.BudgetCap
	key.ExpiresAt = req.ExpiresAt
	if req.Enabled != nil {
		key.Enabled = *req.Enabled
	}
	if req.Metadata != nil {
		key.Metadata = req.Metadata
	}

	if err := h.repo.Update(c.Request.Context(), key); err != nil {
		if errors.Is(err, virtualkey.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "key not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to update key")
		return
	}
	h.invalidateKey(key.ID)

	h.writeKeyAudit(c, "update_key", key.ID, map[string]any{"tenant_id": key.TenantID, "label": key.Label})
	c.JSON(http.StatusOK, dto.VirtualKeyToResponse(key))
}

// @sk-task api-self-service: Rotate replaces the key secret, returning it once (AC-001)
//
// Rotate generates a fresh secret for an existing key, replaces only the stored
// hash, and returns the plaintext exactly once. The key id, scopes, budget and
// expiry are preserved.
func (h *VirtualKeyHandler) Rotate(c *gin.Context) {
	key, err := h.repo.GetById(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, virtualkey.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "key not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to get key")
		return
	}

	raw, err := virtualkey.NewSecret()
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to generate secret")
		return
	}

	key.KeyHash = virtualkey.KeyHash(raw)
	key.UpdatedAt = time.Now().UTC()

	if err := h.repo.Update(c.Request.Context(), key); err != nil {
		if errors.Is(err, virtualkey.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "key not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to rotate key")
		return
	}
	h.invalidateKey(key.ID)

	h.writeKeyAudit(c, "rotate_key", key.ID, map[string]any{"tenant_id": key.TenantID, "label": key.Label})

	resp := dto.CreateVirtualKeyResponse{
		VirtualKeyResponse: dto.VirtualKeyToResponse(key),
		Key:                raw,
	}
	c.JSON(http.StatusOK, resp)
}

// @sk-task 300-virtual-keys#T3.1: Delete revokes a key (soft delete) (AC-001)
func (h *VirtualKeyHandler) Delete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, virtualkey.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "key not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to revoke key")
		return
	}
	h.invalidateKey(c.Param("id"))
	h.writeKeyAudit(c, "revoke_key", c.Param("id"), nil)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *VirtualKeyHandler) invalidateKey(keyID string) {
	if h.cache == nil {
		return
	}
	h.cache.InvalidateKey(keyID)
}

func (h *VirtualKeyHandler) writeKeyAudit(c *gin.Context, action, target string, details map[string]any) {
	if h.auditLog == nil {
		return
	}
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
