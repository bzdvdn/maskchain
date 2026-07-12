package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/tenant"
)

const tenantKey = "tenant_slug"

// @sk-task 80-tenant-isolation#T2.1: TenantFromContext helper for handlers (AC-001, AC-002, AC-003, AC-004)
func TenantFromContext(c *gin.Context) (string, bool) {
	v, ok := c.Get(tenantKey)
	if !ok {
		return "", false
	}
	slug, ok := v.(string)
	return slug, ok
}

var publicPaths = map[string]bool{
	"/health": true,
	"/ready":  true,
	"/live":   true,
	"/metrics": true,
}

// @sk-task 80-tenant-isolation#T2.2: Skip auth for public paths (AC-002)
func isPublicPath(path string) bool {
	return publicPaths[path]
}

// @sk-task 80-tenant-isolation#T2.1: Multi-header auth middleware (AC-001, AC-002, AC-003, AC-004)
func Auth(repo tenant.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			c.Next()
			return
		}
		allTenants := repo.All()
		if len(allTenants) == 0 {
			c.Next()
			return
		}
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		t, ok := authenticate(c, repo)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
			return
		}
		c.Set(tenantKey, t.Slug())
		c.Next()
	}
}

type candidate struct {
	header string
	key    string
}

// @sk-task 80-tenant-isolation#T2.1: Collect candidate pairs for authentication (AC-001, AC-004)
func collectCandidates(c *gin.Context, repo tenant.Repository) []candidate {
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
	for _, t := range repo.All() {
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

func authenticate(c *gin.Context, repo tenant.Repository) (*tenant.Tenant, bool) {
	for _, cand := range collectCandidates(c, repo) {
		if cand.key == "" {
			continue
		}
		t, ok := repo.FindByAPIKey(cand.key)
		if !ok {
			continue
		}
		if t.AuthHeader() != cand.header {
			AbortWithError(c, http.StatusUnauthorized, ErrorCodeUnauthorized, "unauthorized")
			return nil, false
		}
		return t, true
	}
	return nil, false
}
