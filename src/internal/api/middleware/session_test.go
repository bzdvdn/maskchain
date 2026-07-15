package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

type mockSessionStoreMW struct {
	sessions map[string]*session.Session
}

func newMockSessionStoreMW() *mockSessionStoreMW {
	return &mockSessionStoreMW{sessions: make(map[string]*session.Session)}
}

func (m *mockSessionStoreMW) Save(ctx context.Context, s *session.Session) error {
	if _, exists := m.sessions[s.SessionID]; exists {
		return session.ErrSessionConflict
	}
	m.sessions[s.SessionID] = s
	return nil
}

func (m *mockSessionStoreMW) Get(ctx context.Context, tenantID, sessionID string) (*session.Session, error) {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return nil, session.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockSessionStoreMW) IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return session.ErrSessionNotFound
	}
	s.TokenCount += tokens
	s.MessageCount += messages
	s.TotalMasks += totalMasks
	s.DictMaskCount += dictMaskCount
	s.PIIMaskCount += piiMaskCount
	s.PreprocessorCount += preprocessorCount
	return nil
}

func (m *mockSessionStoreMW) ExtendTTL(ctx context.Context, tenantID, sessionID string, newExpiresAt time.Time) error {
	return session.ErrSessionNotFound
}

func (m *mockSessionStoreMW) Close(ctx context.Context, tenantID, sessionID string) error {
	return session.ErrSessionNotFound
}

func (m *mockSessionStoreMW) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockSessionStoreMW) ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*session.ListResult, error) {
	return &session.ListResult{Items: []session.Session{}, Total: 0, Page: int(page), Limit: int(limit)}, nil
}

func setupSessionMWTest(t *testing.T) (*gin.Engine, *mockSessionStoreMW, *entity.Tenant) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ms := newMockSessionStoreMW()
	uc := session.NewSessionUseCase(ms)
	cfg := &config.SessionConfig{
		DefaultTTL: 30 * time.Minute,
		MaxTTL:     24 * time.Hour,
	}
	engine := gin.New()
	slug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(slug, "Test Tenant", "Authorization", nil)

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", tenant)
		c.Next()
	})
	engine.Use(SessionMiddleware(uc, cfg, zap.NewNop()))
	engine.POST("/test", func(c *gin.Context) {
		s, ok := SessionFromContext(c)
		if ok && s != nil {
			c.String(http.StatusOK, "session:%s", s.SessionID)
		} else {
			c.String(http.StatusOK, "no-session")
		}
	})
	return engine, ms, tenant
}

// @sk-test sessions#T4.3: TestSessionMiddlewareCreatesSession (AC-010)
func TestSessionMiddlewareCreatesSession(t *testing.T) {
	engine, ms, _ := setupSessionMWTest(t)

	body := bytes.NewBufferString(`{"model":"gpt-4"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("X-Session-ID", "0190f3a6-7b8c-7d4e-9f01-23456789abcd")
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if _, exists := ms.sessions["0190f3a6-7b8c-7d4e-9f01-23456789abcd"]; !exists {
		t.Error("expected session to be created in store")
	}
	if w.Body.String() != "session:0190f3a6-7b8c-7d4e-9f01-23456789abcd" {
		t.Errorf("expected session in response, got %s", w.Body.String())
	}
	if w.Header().Get("X-Session-ID") != "0190f3a6-7b8c-7d4e-9f01-23456789abcd" {
		t.Error("expected X-Session-ID response header")
	}
}

// @sk-test sessions#T4.3: TestSessionMiddlewareGetsExisting (AC-010)
func TestSessionMiddlewareGetsExisting(t *testing.T) {
	engine, ms, tenant := setupSessionMWTest(t)
	tenantID := tenant.Slug().String()
	uc := session.NewSessionUseCase(ms)
	_, err := uc.Create(context.Background(), "existing-sess", tenantID, "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("create existing session: %v", err)
	}

	body := bytes.NewBufferString(`{"model":"gpt-4"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("X-Session-ID", "existing-sess")
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "session:existing-sess" {
		t.Errorf("expected existing session, got %s", w.Body.String())
	}
}

// @sk-test sessions#T4.3: TestSessionMiddlewareNoHeader (AC-010)
func TestSessionMiddlewareNoHeader(t *testing.T) {
	engine, _, _ := setupSessionMWTest(t)

	body := bytes.NewBufferString(`{"model":"gpt-4"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "no-session" {
		t.Errorf("expected no-session, got %s", w.Body.String())
	}
}

// @sk-test sessions#T4.3: TestSessionMiddlewareWithShieldMiddlewareIncrement (AC-002, AC-010)
func TestSessionMiddlewareWithShieldMiddlewareIncrement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ms := newMockSessionStoreMW()
	uc := session.NewSessionUseCase(ms)
	cfg := &config.SessionConfig{
		DefaultTTL: 30 * time.Minute,
	}
	slug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(slug, "Test Tenant", "Authorization", nil)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", tenant)
		c.Next()
	})
	engine.Use(SessionMiddleware(uc, cfg, zap.NewNop()))
	engine.Use(ShieldMiddleware(nil, &config.ShieldConfig{}, zap.NewNop(), uc))
	engine.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	body := bytes.NewBufferString(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("X-Session-ID", "increment-test-1")
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	sess, exists := ms.sessions["increment-test-1"]
	if !exists {
		t.Fatal("expected session to exist")
	}
	if sess.MessageCount != 1 {
		t.Errorf("expected MessageCount=1, got %d", sess.MessageCount)
	}
}
