package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

func setupAuthTest() (*entity.Tenant, []*entity.Tenant) {
	slugA, _ := value.NewTenantSlug("alpha")
	slugB, _ := value.NewTenantSlug("beta")
	slugC, _ := value.NewTenantSlug("gamma")

	ta := entity.NewTenant(slugA, "Alpha", "Authorization")
	tb := entity.NewTenant(slugB, "Beta", "X-Mask-Authorization")
	tc := entity.NewTenant(slugC, "Gamma", "X-Custom")

	return ta, []*entity.Tenant{ta, tb, tc}
}

// @sk-test tenant-profile-sync#T2.1: TestTenantFromContext returns entity (AC-005)
func TestTenantFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	ta, tenants := setupAuthTest()
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{fakeKey("", "alpha", nil, nil)}}
	repo.keys[0].KeyHash = virtualkey.KeyHash("sk-abc")

	engine.Use(VirtualKeyAuth(repo, NewTenantProvider(tenants)))
	engine.GET("/api/v1/profiles", func(c *gin.Context) {
		got, ok := TenantFromContext(c)
		if !ok {
			t.Error("expected tenant in context")
			return
		}
		if got.Slug() != ta.Slug() {
			t.Errorf("expected %s, got %s", ta.Slug().String(), got.Slug().String())
		}
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer sk-abc")
	engine.ServeHTTP(w, req)
}
