package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

func virtualKeyAuthTestRequest(method, path, header, value string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	_, tenants := setupAuthTest()
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{}}
	// real hashes for the test keys
	keyA := fakeKey("", "alpha", nil, nil)
	keyA.KeyHash = virtualkey.KeyHash("sk-abc")
	keyB := fakeKey("", "beta", nil, nil)
	keyB.KeyHash = virtualkey.KeyHash("mk-xyz")
	repo.keys = append(repo.keys, keyA, keyB)

	engine.Use(VirtualKeyAuth(repo, NewTenantProvider(tenants)))
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

// @sk-test 300-virtual-keys#T2.3: Valid bearer key resolves tenant via hash lookup (AC-001)
func TestVirtualKeyAuthValidBearer(t *testing.T) {
	w := virtualKeyAuthTestRequest("GET", "/api/v1/profiles", "Authorization", "Bearer sk-abc")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 300-virtual-keys#T2.3: Unknown key is rejected (AC-001)
func TestVirtualKeyAuthUnknownKey(t *testing.T) {
	w := virtualKeyAuthTestRequest("GET", "/api/v1/profiles", "Authorization", "Bearer unknown")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test 300-virtual-keys#T2.3: Key in the wrong header is rejected (AC-001)
func TestVirtualKeyAuthWrongHeader(t *testing.T) {
	w := virtualKeyAuthTestRequest("GET", "/api/v1/profiles", "Authorization", "Bearer mk-xyz")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test 300-virtual-keys#T2.3: Public paths skip auth (AC-001)
func TestVirtualKeyAuthPublicPath(t *testing.T) {
	w := virtualKeyAuthTestRequest("GET", "/health", "", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for public path, got %d", w.Code)
	}
}

// @sk-test 300-virtual-keys#T2.1: Tenant and virtual key are placed in context (AC-003)
func TestVirtualKeyAuthSetsTenantAndKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	ta, tenants := setupAuthTest()
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{fakeKey("", "alpha", nil, nil)}}
	repo.keys[0].KeyHash = virtualkey.KeyHash("sk-abc")

	engine.Use(VirtualKeyAuth(repo, NewTenantProvider(tenants)))
	engine.GET("/api/v1/profiles", func(c *gin.Context) {
		gotT, ok := TenantFromContext(c)
		if !ok {
			t.Error("expected tenant in context")
			return
		}
		if gotT.Slug() != ta.Slug() {
			t.Errorf("expected %s, got %s", ta.Slug().String(), gotT.Slug().String())
		}
		vk, ok := VirtualKeyFromContext(c)
		if !ok || vk == nil || vk.TenantID != "alpha" {
			t.Errorf("expected virtual key in context, got %v", vk)
		}
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer sk-abc")
	engine.ServeHTTP(w, req)
}
