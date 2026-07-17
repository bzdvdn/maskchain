package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

type mockSessionStore struct {
	sessions map[string]*session.Session
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{sessions: make(map[string]*session.Session)}
}

func (m *mockSessionStore) Save(ctx context.Context, s *session.Session) error {
	if _, exists := m.sessions[s.SessionID]; exists {
		return session.ErrSessionConflict
	}
	m.sessions[s.SessionID] = s
	return nil
}

func (m *mockSessionStore) Get(ctx context.Context, tenantID, sessionID string) (*session.Session, error) {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return nil, session.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockSessionStore) IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error {
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

func (m *mockSessionStore) ExtendTTL(ctx context.Context, tenantID, sessionID string, newExpiresAt time.Time) error {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return session.ErrSessionNotFound
	}
	s.ExpiresAt = newExpiresAt
	return nil
}

func (m *mockSessionStore) Close(ctx context.Context, tenantID, sessionID string) error {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return session.ErrSessionNotFound
	}
	s.Status = session.SessionStatusClosed
	return nil
}

func (m *mockSessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	var deleted int64
	for id, s := range m.sessions {
		if s.Status == session.SessionStatusExpired || time.Now().After(s.ExpiresAt) {
			delete(m.sessions, id)
			deleted++
		}
	}
	return deleted, nil
}

func (m *mockSessionStore) ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*session.ListResult, error) {
	var items []session.Session
	for _, s := range m.sessions {
		if s.TenantID == tenantID {
			items = append(items, *s)
		}
	}
	if items == nil {
		items = []session.Session{}
	}
	return &session.ListResult{Items: items, Total: len(items), Page: int(page), Limit: int(limit)}, nil
}

func (m *mockSessionStore) ListAll(ctx context.Context, page, limit int32) (*session.ListResult, error) {
	var items []session.Session
	for _, s := range m.sessions {
		items = append(items, *s)
	}
	if items == nil {
		items = []session.Session{}
	}
	return &session.ListResult{Items: items, Total: len(items), Page: int(page), Limit: int(limit)}, nil
}

func sessionHandlerTestTenant(t *testing.T) *entity.Tenant {
	t.Helper()
	slug, err := value.NewTenantSlug("test-tenant")
	if err != nil {
		t.Fatalf("NewTenantSlug: %v", err)
	}
	return entity.NewTenant(slug, "Test Tenant", "Authorization", nil)
}

func setupSessionHandlerTest(t *testing.T) (*SessionHandler, *mockSessionStore, *config.SessionConfig) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ms := newMockSessionStore()
	uc := session.NewSessionUseCase(ms)
	cfg := &config.SessionConfig{
		DefaultTTL: 30 * time.Minute,
		MaxTTL:     24 * time.Hour,
	}
	handler := NewSessionHandler(uc, cfg)
	return handler, ms, cfg
}

func setTenantInContext(c *gin.Context, tenant *entity.Tenant) {
	c.Set("tenant", tenant)
}

// @sk-test sessions#T2.4: TestSessionHandler_Create (AC-001)
func TestSessionHandler_Create(t *testing.T) {
	handler, _, _ := setupSessionHandlerTest(t)
	tenant := sessionHandlerTestTenant(t)

	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		body := bytes.NewBufferString(`{"model":"gpt-4"}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sessions", body)
		c.Request.Header.Set("Content-Type", "application/json")

		handler.HandleCreate(c)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json unmarshal: %v", err)
		}
		if resp["session_id"] == "" {
			t.Error("expected non-empty session_id")
		}
		if resp["tenant_id"] != tenant.Slug().String() {
			t.Errorf("expected tenant_id=%s, got %v", tenant.Slug().String(), resp["tenant_id"])
		}
		if resp["model"] != "gpt-4" {
			t.Errorf("expected model=gpt-4, got %v", resp["model"])
		}
		if resp["status"] != "active" {
			t.Errorf("expected status=active, got %v", resp["status"])
		}
	})

	t.Run("missing_tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := bytes.NewBufferString(`{"model":"gpt-4"}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sessions", body)
		c.Request.Header.Set("Content-Type", "application/json")

		handler.HandleCreate(c)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing_model", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		body := bytes.NewBufferString(`{}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sessions", body)
		c.Request.Header.Set("Content-Type", "application/json")

		handler.HandleCreate(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// @sk-test sessions#T2.4: TestSessionHandler_Get (AC-003)
func TestSessionHandler_Get(t *testing.T) {
	handler, ms, _ := setupSessionHandlerTest(t)
	tenant := sessionHandlerTestTenant(t)
	tenantID := tenant.Slug().String()

	uc := session.NewSessionUseCase(ms)
	_, err := uc.Create(context.Background(), "sess-1", tenantID, "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sessions/sess-1", nil)
		c.Params = []gin.Param{{Key: "id", Value: "sess-1"}}

		handler.HandleGet(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json unmarshal: %v", err)
		}
		if resp["session_id"] != "sess-1" {
			t.Errorf("expected sess-1, got %v", resp["session_id"])
		}
	})

	t.Run("not_found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent", nil)
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

		handler.HandleGet(c)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing_tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sessions/sess-1", nil)
		c.Params = []gin.Param{{Key: "id", Value: "sess-1"}}

		handler.HandleGet(c)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// @sk-test sessions#T2.4: TestSessionHandler_List (AC-004)
func TestSessionHandler_List(t *testing.T) {
	handler, ms, _ := setupSessionHandlerTest(t)
	tenant := sessionHandlerTestTenant(t)
	tenantID := tenant.Slug().String()

	uc := session.NewSessionUseCase(ms)
	for i := 0; i < 3; i++ {
		id := "list-sess-" + string(rune('a'+i))
		_, err := uc.Create(context.Background(), id, tenantID, "gpt-4", 30*time.Minute)
		if err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page=1&limit=10", nil)

		handler.HandleList(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json unmarshal: %v", err)
		}
		total, ok := resp["total"].(float64)
		if !ok || int(total) != 3 {
			t.Errorf("expected total=3, got %v", resp["total"])
		}
	})

	t.Run("missing_tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)

		handler.HandleList(c)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// @sk-test sessions#T2.4: TestSessionHandler_Extend (AC-005)
func TestSessionHandler_Extend(t *testing.T) {
	handler, ms, _ := setupSessionHandlerTest(t)
	tenant := sessionHandlerTestTenant(t)
	tenantID := tenant.Slug().String()

	uc := session.NewSessionUseCase(ms)
	_, err := uc.Create(context.Background(), "ext-sess", tenantID, "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		body := bytes.NewBufferString(`{"ttl_seconds":3600}`)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/ext-sess/extend", body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: "ext-sess"}}

		handler.HandleExtend(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("not_found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		body := bytes.NewBufferString(`{"ttl_seconds":3600}`)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/nonexistent/extend", body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

		handler.HandleExtend(c)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// @sk-test sessions#T2.4: TestSessionHandler_Close (AC-006)
func TestSessionHandler_Close(t *testing.T) {
	handler, ms, _ := setupSessionHandlerTest(t)
	tenant := sessionHandlerTestTenant(t)
	tenantID := tenant.Slug().String()

	uc := session.NewSessionUseCase(ms)
	_, err := uc.Create(context.Background(), "close-sess", tenantID, "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/close-sess", nil)
		c.Params = []gin.Param{{Key: "id", Value: "close-sess"}}

		handler.HandleClose(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json unmarshal: %v", err)
		}
		if resp["status"] != "closed" {
			t.Errorf("expected status=closed, got %v", resp["status"])
		}
	})

	t.Run("already_closed", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/close-sess", nil)
		c.Params = []gin.Param{{Key: "id", Value: "close-sess"}}

		handler.HandleClose(c)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("not_found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setTenantInContext(c, tenant)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/nonexistent", nil)
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

		handler.HandleClose(c)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing_tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/close-sess", nil)
		c.Params = []gin.Param{{Key: "id", Value: "close-sess"}}

		handler.HandleClose(c)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})
}
