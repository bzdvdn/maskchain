package middleware

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

const tenantKey = "tenant"

// @sk-task tenant-profile-sync#T2.1: TenantFromContext returns Tenant entity from context
func TenantFromContext(c *gin.Context) (*entity.Tenant, bool) {
	v, ok := c.Get(tenantKey)
	if !ok {
		return nil, false
	}
	t, ok := v.(*entity.Tenant)
	return t, ok
}

var publicPaths = map[string]bool{
	"/health":  true,
	"/ready":   true,
	"/live":    true,
	"/metrics": true,
}

// @sk-task 80-tenant-isolation#T2.2: Skip auth for public paths (AC-002)
func isPublicPath(path string) bool {
	return publicPaths[path]
}

// TenantProvider provides thread-safe access to tenants with hot-reload support.
type TenantProvider struct {
	mu      sync.RWMutex
	tenants []*entity.Tenant
}

func NewTenantProvider(tenants []*entity.Tenant) *TenantProvider {
	return &TenantProvider{tenants: tenants}
}

func (p *TenantProvider) Get() []*entity.Tenant {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tenants
}

func (p *TenantProvider) Update(tenants []*entity.Tenant) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tenants = tenants
}

// @sk-task tenant-profile-sync#T2.1: Multi-header auth middleware using TenantResolver (AC-002, AC-005)
func Auth(provider *TenantProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenants := provider.Get()
		if len(tenants) == 0 {
			c.Next()
			return
		}
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		t, ok := authenticate(c, tenants)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
			return
		}
		c.Set(tenantKey, t)
		c.Next()
	}
}

type candidate struct {
	header string
	key    string
}

// @sk-task tenant-profile-sync#T2.1: Collect candidate pairs for authentication
func collectCandidates(c *gin.Context, tenants []*entity.Tenant) []candidate {
	var candidates []candidate

	authz := c.GetHeader("Authorization")
	if authz != "" && strings.HasPrefix(authz, "Bearer ") {
		candidates = append(candidates, candidate{header: "Authorization", key: strings.TrimPrefix(authz, "Bearer ")})
	}

	defaultVal := c.GetHeader("X-Mask-Authorization")
	if defaultVal != "" {
		candidates = append(candidates, candidate{header: "X-Mask-Authorization", key: defaultVal})
	}

	seen := map[string]bool{"Authorization": true, "X-Mask-Authorization": true}
	for _, t := range tenants {
		h := t.AuthHeader()
		if seen[h] {
			continue
		}
		seen[h] = true
		val := c.GetHeader(h)
		if val != "" {
			candidates = append(candidates, candidate{header: h, key: val})
		}
	}
	return candidates
}

func authenticate(c *gin.Context, tenants []*entity.Tenant) (*entity.Tenant, bool) {
	for _, cand := range collectCandidates(c, tenants) {
		if cand.key == "" {
			continue
		}
		for _, t := range tenants {
			for _, k := range t.APIKeys() {
				if k == cand.key {
					if t.AuthHeader() != cand.header {
						AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
						return nil, false
					}
					return t, true
				}
			}
		}
	}
	return nil, false
}
