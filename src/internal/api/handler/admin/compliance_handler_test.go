package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/compliance"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type fakeTenantRepo struct {
	bySlug map[string]*entity.Tenant
}

func newFakeTenantRepo(tenants ...*entity.Tenant) *fakeTenantRepo {
	r := &fakeTenantRepo{bySlug: make(map[string]*entity.Tenant)}
	for _, t := range tenants {
		r.bySlug[t.Slug().String()] = t
	}
	return r
}

func (r *fakeTenantRepo) List(ctx context.Context) ([]*entity.Tenant, error) {
	out := make([]*entity.Tenant, 0, len(r.bySlug))
	for _, t := range r.bySlug {
		out = append(out, t)
	}
	return out, nil
}

// @sk-task ui-production-readiness#T7.1: DB-level pagination/search (AC-008)
func (r *fakeTenantRepo) ListPaged(ctx context.Context, limit, offset int, search string) ([]*entity.Tenant, int, error) {
	all, _ := r.List(ctx)
	filtered := make([]*entity.Tenant, 0, len(all))
	for _, t := range all {
		if search == "" || containsFold(t.Slug().String(), search) || containsFold(t.Name(), search) {
			filtered = append(filtered, t)
		}
	}
	total := len(filtered)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func (r *fakeTenantRepo) Get(ctx context.Context, slug value.TenantSlug) (*entity.Tenant, error) {
	t, ok := r.bySlug[slug.String()]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (r *fakeTenantRepo) Create(ctx context.Context, t *entity.Tenant) error {
	r.bySlug[t.Slug().String()] = t
	return nil
}

func (r *fakeTenantRepo) Update(ctx context.Context, t *entity.Tenant) error {
	if _, ok := r.bySlug[t.Slug().String()]; !ok {
		return shielderrors.ErrTenantNotFound
	}
	r.bySlug[t.Slug().String()] = t
	return nil
}

func (r *fakeTenantRepo) Delete(ctx context.Context, slug value.TenantSlug) error { return nil }

func (r *fakeTenantRepo) GetDictionaries(ctx context.Context, slug value.TenantSlug) ([]*dictionary.Dictionary, error) {
	return nil, nil
}

func (r *fakeTenantRepo) UpdateDictionaries(ctx context.Context, slug value.TenantSlug, dicts []*dictionary.Dictionary) error {
	return nil
}

func testComplianceRegistry(t *testing.T) *compliance.Registry {
	t.Helper()
	catalog := compliance.NewCatalog(
		[]entity.DetectorType{entity.DetectorTypeRegex, entity.DetectorTypeDictionary, entity.DetectorTypePromptInjection},
		[]entity.Reaction{entity.ReactionAllow, entity.ReactionBlock, entity.ReactionReview, entity.ReactionLog},
	)
	reg, errs := compliance.LoadPacksFromDir("../../../domain/compliance/testdata", catalog)
	if len(errs) != 1 {
		t.Fatalf("expected 1 load error, got %v", errs)
	}
	return reg
}

func newComplianceHandler(t *testing.T) (*ComplianceHandler, *fakeTenantRepo) {
	t.Helper()
	repo := newFakeTenantRepo()
	s, _ := value.NewTenantSlug("acme")
	repo.Create(context.Background(), entity.NewTenant(s, "acme", "X-Auth"))
	h := NewComplianceHandler(testComplianceRegistry(t), repo)
	return h, repo
}

func TestComplianceHandler_ApplyPack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, repo := newComplianceHandler(t)

	router := gin.New()
	router.POST("/api/v1/tenants/:slug/compliance/apply", h.HandleApplyPack)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/acme/compliance/apply",
		bytes.NewBufferString(`{"pack_key":"HIPAA"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	s, _ := value.NewTenantSlug("acme")
	tenant, _ := repo.Get(context.Background(), s)
	if tenant == nil || !tenant.PIIConfig().Enabled {
		t.Fatal("expected tenant pii config enabled after apply")
	}
}

func TestComplianceHandler_ApplyUnknownPack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newComplianceHandler(t)

	router := gin.New()
	router.POST("/api/v1/tenants/:slug/compliance/apply", h.HandleApplyPack)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/acme/compliance/apply",
		bytes.NewBufferString(`{"pack_key":"NOPE"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestComplianceHandler_Report(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newComplianceHandler(t)

	router := gin.New()
	router.GET("/api/v1/tenants/:slug/compliance/report", h.HandleComplianceReport)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/acme/compliance/report?pack=HIPAA", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); len(body) == 0 {
		t.Fatal("expected non-empty report body")
	}
}

func TestComplianceHandler_ReportMissingPackParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newComplianceHandler(t)

	router := gin.New()
	router.GET("/api/v1/tenants/:slug/compliance/report", h.HandleComplianceReport)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/acme/compliance/report", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
