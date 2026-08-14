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

// @sk-task 300-virtual-keys#T2.1: VirtualKeyAuth authenticates via SHA-256 key lookup (AC-001)
// @sk-task 403-key-at-rest-encryption#T3.2: VirtualKeyAuth resolves from cache first (AC-005, AC-006)
//
// VirtualKeyAuth handles the operation. When a cache is provided it resolves
// tenants from the in-process index, falling back to the repository on a cache
// miss; a nil cache keeps the previous repo-only behavior.
func VirtualKeyAuth(repo virtualkey.VirtualKeyRepository, provider *TenantProvider, cache ...*VirtualKeyCache) gin.HandlerFunc {
	var vkCache *VirtualKeyCache
	if len(cache) > 0 {
		vkCache = cache[0]
	}
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

		key, vk := authenticateVirtualKey(c, repo, vkCache, tenants)
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

// lookupVirtualKey fetches a key by hash from the cache when present, and falls
// back to the repository on a miss. Lazy cache population avoids repeated DB
// round-trips for the same key between refreshes.
func lookupVirtualKey(c *gin.Context, repo virtualkey.VirtualKeyRepository, cache *VirtualKeyCache, hash string) (*virtualkey.VirtualKey, bool) {
	if cache != nil {
		if vk, ok := cache.Get(hash); ok {
			return vk, true
		}
	}
	vk, err := repo.FindByKeyHash(c.Request.Context(), hash)
	if err != nil || vk == nil {
		return nil, false
	}
	if cache != nil {
		cache.Set(hash, vk)
	}
	return vk, true
}

// authenticateVirtualKey resolves a tenant by hashed candidate keys. It mirrors
// the header rules of the legacy in-memory Auth middleware so existing clients
// do not change their headers.
func authenticateVirtualKey(c *gin.Context, repo virtualkey.VirtualKeyRepository, cache *VirtualKeyCache, tenants []*entity.Tenant) (*entity.Tenant, *virtualkey.VirtualKey) {
	for _, cand := range collectCandidates(c, tenants) {
		if cand.key == "" {
			continue
		}
		vk, ok := lookupVirtualKey(c, repo, cache, virtualkey.KeyHash(cand.key))
		if !ok {
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
