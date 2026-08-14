package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 402-zero-retention-mode#T2.3: CreateTenant accepts retention_mode (AC-002, AC-010)
func TestTenantHandler_CreateWithRetentionMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeTenantRepo()
	h := NewTenantHandler(repo, nil, nil)

	router := gin.New()
	router.POST("/api/v1/tenants", h.CreateTenant)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants",
		bytes.NewBufferString(`{"slug":"acme","name":"Acme","retention_mode":"none"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	s, _ := value.NewTenantSlug("acme")
	tenant, _ := repo.Get(context.Background(), s)
	if tenant == nil {
		t.Fatal("expected tenant to be created")
	}
	if tenant.RetentionMode() != value.RetentionModeNone {
		t.Errorf("RetentionMode = %q, want %q", tenant.RetentionMode(), value.RetentionModeNone)
	}
}

// @sk-test 402-zero-retention-mode#T2.3: UpdateTenant preserves previous mode when omitted (AC-007)
func TestTenantHandler_UpdatePreservesRetentionMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s, _ := value.NewTenantSlug("acme")
	repo := newFakeTenantRepo()
	_ = repo.Create(context.Background(), entity.NewTenant(s, "Acme", "X-Auth",
		entity.WithTenantRetentionMode(value.RetentionModeNone)))
	hh := NewTenantHandler(repo, nil, nil)

	router := gin.New()
	router.PUT("/api/v1/tenants/:slug", hh.UpdateTenant)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/tenants/acme",
		bytes.NewBufferString(`{"name":"Acme2"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	tenant, _ := repo.Get(context.Background(), s)
	if tenant.RetentionMode() != value.RetentionModeNone {
		t.Errorf("RetentionMode = %q, want preserved %q", tenant.RetentionMode(), value.RetentionModeNone)
	}
}

// @sk-test 402-zero-retention-mode#T2.3: UpdateTenant rejects invalid mode and keeps previous (AC-007)
func TestTenantHandler_UpdateInvalidRetentionMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s, _ := value.NewTenantSlug("acme")
	repo := newFakeTenantRepo()
	_ = repo.Create(context.Background(), entity.NewTenant(s, "Acme", "X-Auth",
		entity.WithTenantRetentionMode(value.RetentionModeFull)))
	hh := NewTenantHandler(repo, nil, nil)

	router := gin.New()
	router.PUT("/api/v1/tenants/:slug", hh.UpdateTenant)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/tenants/acme",
		bytes.NewBufferString(`{"name":"Acme2","retention_mode":"redacted"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	tenant, _ := repo.Get(context.Background(), s)
	if tenant.RetentionMode() != value.RetentionModeFull {
		t.Errorf("RetentionMode = %q, want unchanged %q", tenant.RetentionMode(), value.RetentionModeFull)
	}
}
