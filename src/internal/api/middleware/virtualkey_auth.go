package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

const virtualKeyContextKey = "virtual_key"

// @sk-task 300-virtual-keys#T2.1: VirtualKeyFromContext returns the virtual key from context (AC-001)
func VirtualKeyFromContext(c *gin.Context) (*virtualkey.VirtualKey, bool) {
	v, ok := c.Get(virtualKeyContextKey)
	if !ok {
		return nil, false
	}
	k, ok := v.(*virtualkey.VirtualKey)
	return k, ok
}

// @sk-task 300-virtual-keys#T2.1: VirtualKeyAuth authenticates via DB-first SHA-256 key lookup (AC-001)
//
// VirtualKeyAuth handles the operation.
func VirtualKeyAuth(repo virtualkey.VirtualKeyRepository, provider *TenantProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		tenants := provider.Get()
		if len(tenants) == 0 {
			c.Next()
			return
		}

		key, vk := authenticateVirtualKey(c, repo, tenants)
		if key == nil {
			AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
			return
		}
		c.Set(tenantKey, key)
		if vk != nil {
			c.Set(virtualKeyContextKey, vk)
		}
		c.Next()
	}
}

// authenticateVirtualKey resolves a tenant by hashed candidate keys. It mirrors
// the header rules of the legacy in-memory Auth middleware so existing clients
// do not change their headers.
func authenticateVirtualKey(c *gin.Context, repo virtualkey.VirtualKeyRepository, tenants []*entity.Tenant) (*entity.Tenant, *virtualkey.VirtualKey) {
	for _, cand := range collectCandidates(c, tenants) {
		if cand.key == "" {
			continue
		}
		vk, err := repo.FindByKeyHash(c.Request.Context(), virtualkey.KeyHash(cand.key))
		if err != nil || vk == nil {
			continue
		}
		if !vk.Valid(time.Now()) {
			continue
		}
		for _, t := range tenants {
			if t.Slug().String() != vk.TenantID {
				continue
			}
			if t.AuthHeader() != cand.header {
				AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
				return nil, nil
			}
			return t, vk
		}
	}
	return nil, nil
}
