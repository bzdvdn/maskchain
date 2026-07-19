package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-test admin-ui-design#T4.1: TestHandleLoginSuccess (AC-001)
func TestHandleLoginSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestAuthHandler("admin", "test123")

	w := doLoginRequest(t, h, `{"username":"admin","password":"test123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.ExpiresAt == 0 {
		t.Error("expected non-zero expires_at")
	}

	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "admin_token" {
			found = true
			if c.Value == "" {
				t.Error("expected non-empty cookie value")
			}
			break
		}
	}
	if !found {
		t.Error("expected admin_token cookie")
	}
}

// @sk-test admin-ui-design#T4.1: TestHandleLoginInvalidCredentials (AC-001)
func TestHandleLoginInvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestAuthHandler("admin", "test123")

	w := doLoginRequest(t, h, `{"username":"admin","password":"wrong"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// @sk-test admin-ui-design#T4.1: TestHandleLoginNotConfigured (AC-001)
func TestHandleLoginNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestAuthHandler("", "")

	w := doLoginRequest(t, h, `{"username":"admin","password":"test123"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// @sk-test admin-ui-design#T4.1: TestHandleLoginInvalidBody (AC-001)
func TestHandleLoginInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestAuthHandler("admin", "test123")

	w := doLoginRequest(t, h, `{invalid json}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// @sk-test admin-ui-design#T4.1: TestHandleLogout (AC-001)
func TestHandleLogout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestAuthHandler("admin", "test123")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/logout", nil)
	ctx.Request.AddCookie(&http.Cookie{Name: "admin_token", Value: "some-token"})

	h.HandleLogout(ctx)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// -- helpers --

func newTestAuthHandler(username, password string) *AdminAuthHandler {
	usecase := admin_session.NewAdminSessionUseCase(&mockAdminSessionStore{})
	cfg := &config.AdminConfig{Username: username, Password: password}
	return NewAdminAuthHandler(usecase, cfg)
}

func doLoginRequest(t *testing.T, h *AdminAuthHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	h.HandleLogin(ctx)
	return w
}

type mockAdminSessionStore struct{}

func (m *mockAdminSessionStore) Save(_ context.Context, _ *admin_session.AdminSession) error {
	return nil
}
func (m *mockAdminSessionStore) GetByTokenHash(_ context.Context, _ string) (*admin_session.AdminSession, error) {
	sess, _, _ := admin_session.NewAdminSession("admin", 30*time.Minute)
	return sess, nil
}
func (m *mockAdminSessionStore) Delete(_ context.Context, _ string) error       { return nil }
func (m *mockAdminSessionStore) DeleteExpired(_ context.Context) (int64, error) { return 0, nil }
