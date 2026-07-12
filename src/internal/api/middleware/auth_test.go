package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/tenant"
	tenantrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/tenant"
)

func setupAuthTest() (*tenant.Tenant, *tenant.Tenant, tenant.Repository) {
	k1, _ := tenant.NewAPIKey("sk-abc")
	k2, _ := tenant.NewAPIKey("mk-xyz")
	k3, _ := tenant.NewAPIKey("custom-token")

	ta := tenant.NewTenant("alpha", "Alpha", "p1", []tenant.APIKey{k1}, "Authorization", "bearer")
	tb := tenant.NewTenant("beta", "Beta", "p2", []tenant.APIKey{k2}, "", "")
	tc := tenant.NewTenant("gamma", "Gamma", "p3", []tenant.APIKey{k3}, "X-Custom", "raw")

	repo, _ := tenantrepo.NewInMemoryRepository([]*tenant.Tenant{ta, tb, tc})
	return ta, tb, repo
}

func authTestRequest(method, path, header, value string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	_, _, repo := setupAuthTest()
	engine.Use(Auth(repo))
	engine.GET("/api/v1/profiles", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	engine.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(method, path, nil)
	if header != "" {
		req.Header.Set(header, value)
	}
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test 80-tenant-isolation#T4.2: TestAuthValidBearer (AC-001)
func TestAuthValidBearer(t *testing.T) {
	w := authTestRequest("GET", "/api/v1/profiles", "Authorization", "Bearer sk-abc")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 80-tenant-isolation#T4.2: TestAuthMissingHeader (AC-002)
func TestAuthMissingHeader(t *testing.T) {
	w := authTestRequest("GET", "/api/v1/profiles", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test 80-tenant-isolation#T4.2: TestAuthInvalidKey (AC-003)
func TestAuthInvalidKey(t *testing.T) {
	w := authTestRequest("GET", "/api/v1/profiles", "Authorization", "Bearer unknown-key")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test 80-tenant-isolation#T4.2: TestAuthEmptyToken (AC-001, AC-002)
func TestAuthEmptyToken(t *testing.T) {
	w := authTestRequest("GET", "/api/v1/profiles", "Authorization", "Bearer ")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test 80-tenant-isolation#T4.2: TestAuthDefaultHeader (AC-010)
func TestAuthDefaultHeader(t *testing.T) {
	w := authTestRequest("GET", "/api/v1/profiles", "X-Mask-Authorization", "mk-xyz")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 80-tenant-isolation#T4.2: TestAuthCustomHeader (AC-004)
func TestAuthCustomHeader(t *testing.T) {
	w := authTestRequest("GET", "/api/v1/profiles", "X-Custom", "custom-token")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 80-tenant-isolation#T4.2: TestAuthKeyInWrongHeader (AC-001, AC-003)
func TestAuthKeyInWrongHeader(t *testing.T) {
	w := authTestRequest("GET", "/api/v1/profiles", "Authorization", "Bearer mk-xyz")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test 80-tenant-isolation#T4.2: TestTenantFromContext (AC-001)
func TestTenantFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	_, _, repo := setupAuthTest()
	engine.Use(Auth(repo))
	engine.GET("/api/v1/profiles", func(c *gin.Context) {
		slug, ok := TenantFromContext(c)
		if !ok {
			t.Error("expected tenant in context")
			return
		}
		if slug != "alpha" {
			t.Errorf("expected alpha, got %s", slug)
		}
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer sk-abc")
	engine.ServeHTTP(w, req)
}

// @sk-test 80-tenant-isolation#T4.2: TestAuthPublicPathsSkipped (AC-002)
func TestAuthPublicPathsSkipped(t *testing.T) {
	w := authTestRequest("GET", "/health", "", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for public path, got %d", w.Code)
	}
}
