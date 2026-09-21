package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
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

// @sk-test ui-production-readiness#T5.2: Tenant list pagination/search (AC-008)
func TestListTenantsPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeTenantRepo()
	for _, slug := range []string{"acme", "beta", "gamma"} {
		s, err := value.NewTenantSlug(slug)
		if err != nil {
			t.Fatalf("slug: %v", err)
		}
		if err := repo.Create(context.Background(), entity.NewTenant(s, strings.ToUpper(slug), "X-Auth")); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	h := NewTenantHandler(repo, nil, nil)

	router := gin.New()
	router.GET("/api/v1/tenants", h.ListTenants)

	t.Run("search narrows and reports total", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants?limit=2&offset=0&search=acme", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body struct {
			Data       []dto.TenantResponse `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if body.Pagination.Total != 1 {
			t.Errorf("total = %d, want 1", body.Pagination.Total)
		}
		if len(body.Data) != 1 || body.Data[0].Slug != "acme" {
			t.Errorf("data = %+v, want only acme", body.Data)
		}
	})

	t.Run("limit pages the result and keeps the full total", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants?limit=2&offset=0", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var body struct {
			Data       []dto.TenantResponse `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if body.Pagination.Total != 3 {
			t.Errorf("total = %d, want 3", body.Pagination.Total)
		}
		if len(body.Data) != 2 {
			t.Errorf("len(data) = %d, want 2", len(body.Data))
		}
	})
}
