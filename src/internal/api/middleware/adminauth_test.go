package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
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

// -- AdminSessionAuth tests --

func adminSessionAuthRequest(cookieToken, bearerToken string, mockStore *mockAdminSessionStore) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	useCase := admin_session.NewAdminSessionUseCase(mockStore)
	protected := engine.Group("/api/v1", AdminSessionAuth(useCase))
	protected.GET("/tenants", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/api/v1/tenants", nil)
	if cookieToken != "" {
		req.AddCookie(&http.Cookie{Name: "admin_token", Value: cookieToken})
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionAuthWithCookie (AC-001, AC-004)
func TestAdminSessionAuthWithCookie(t *testing.T) {
	w := adminSessionAuthRequest("valid-token", "", &mockAdminSessionStore{allow: true})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionAuthWithBearer (AC-001, AC-004)
func TestAdminSessionAuthWithBearer(t *testing.T) {
	w := adminSessionAuthRequest("", "valid-token", &mockAdminSessionStore{allow: true})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionAuthWithoutToken (AC-001, AC-004)
func TestAdminSessionAuthWithoutToken(t *testing.T) {
	w := adminSessionAuthRequest("", "", &mockAdminSessionStore{})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionAuthInvalidToken (AC-004)
func TestAdminSessionAuthInvalidToken(t *testing.T) {
	w := adminSessionAuthRequest("nonexistent-token", "", &mockAdminSessionStore{})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// -- mock --

type mockAdminSessionStore struct {
	allow bool
}

func (m *mockAdminSessionStore) Save(_ context.Context, _ *admin_session.AdminSession) error {
	return nil
}
func (m *mockAdminSessionStore) GetByTokenHash(_ context.Context, _ string) (*admin_session.AdminSession, error) {
	if m.allow {
		return &admin_session.AdminSession{Username: "admin", ExpiresAt: time.Now().Add(30 * time.Minute)}, nil
	}
	return nil, admin_session.ErrSessionNotFound
}
func (m *mockAdminSessionStore) Delete(_ context.Context, _ string) error       { return nil }
func (m *mockAdminSessionStore) DeleteExpired(_ context.Context) (int64, error) { return 0, nil }
