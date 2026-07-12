package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

func adminAuthRequest(method, path, token string, debugEnabled bool) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	cfg := &config.DebugConfig{
		Enabled:    debugEnabled,
		AdminToken: "valid-admin-token",
	}

	protected := engine.Group("/debug/pprof", AdminAuth(cfg))
	protected.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("X-Admin-Token", token)
	}
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test 90-production-hardening#T4.3: TestAdminAuthValidToken (<AC-001>)
func TestAdminAuthValidToken(t *testing.T) {
	w := adminAuthRequest("GET", "/debug/pprof/", "valid-admin-token", true)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 90-production-hardening#T4.3: TestAdminAuthMissingToken (<AC-001>)
func TestAdminAuthMissingToken(t *testing.T) {
	w := adminAuthRequest("GET", "/debug/pprof/", "", true)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test 90-production-hardening#T4.3: TestAdminAuthInvalidToken (<AC-001>)
func TestAdminAuthInvalidToken(t *testing.T) {
	w := adminAuthRequest("GET", "/debug/pprof/", "wrong-token", true)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test 90-production-hardening#T4.3: TestAdminAuthDebugDisabled (<AC-001>)
func TestAdminAuthDebugDisabled(t *testing.T) {
	w := adminAuthRequest("GET", "/debug/pprof/", "", false)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (skip auth), got %d", w.Code)
	}
}
