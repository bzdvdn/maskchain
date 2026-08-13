package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
	reg, errs := compliance.LoadPacksFromDir("../../../../../specs/active/401-compliance-packs/testdata", catalog)
	if len(errs) != 1 {
		t.Fatalf("expected 1 load error, got %v", errs)
	}
	return reg
}

func newComplianceHandler(t *testing.T) (*ComplianceHandler, *fakeTenantRepo) {
	t.Helper()
	repo := newFakeTenantRepo()
	s, _ := value.NewTenantSlug("acme")
	repo.Create(context.Background(), entity.NewTenant(s, "acme", "X-Auth", []string{"k"}))
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
