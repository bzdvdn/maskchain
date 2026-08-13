package compliance

import (
	"context"
	"errors"
	"testing"

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

func (r *fakeTenantRepo) Delete(ctx context.Context, slug value.TenantSlug) error {
	delete(r.bySlug, slug.String())
	return nil
}

func (r *fakeTenantRepo) GetDictionaries(ctx context.Context, slug value.TenantSlug) ([]*dictionary.Dictionary, error) {
	t, ok := r.bySlug[slug.String()]
	if !ok {
		return nil, shielderrors.ErrTenantNotFound
	}
	return t.Dictionaries(), nil
}

func (r *fakeTenantRepo) UpdateDictionaries(ctx context.Context, slug value.TenantSlug, dicts []*dictionary.Dictionary) error {
	t, ok := r.bySlug[slug.String()]
	if !ok {
		return shielderrors.ErrTenantNotFound
	}
	t.SetDictionaries(dicts)
	return nil
}

func hipaaRegistry(t *testing.T) *compliance.Registry {
	t.Helper()
	catalog := compliance.NewCatalog(
		[]entity.DetectorType{entity.DetectorTypeRegex, entity.DetectorTypeDictionary, entity.DetectorTypePromptInjection},
		[]entity.Reaction{entity.ReactionAllow, entity.ReactionBlock, entity.ReactionReview, entity.ReactionLog},
	)
	reg, errs := compliance.LoadPacksFromDir("../../../../specs/active/401-compliance-packs/testdata", catalog)
	if len(errs) != 1 {
		t.Fatalf("expected 1 load error, got %v", errs)
	}
	return reg
}

func testTenant(slug string) *entity.Tenant {
	s, _ := value.NewTenantSlug(slug)
	return entity.NewTenant(s, slug, "X-Auth", []string{"key-" + slug}, entity.WithTenantPIIConfig(entity.PIIConfig{}))
}

func TestApplyPackService_AppliesPreset(t *testing.T) {
	reg := hipaaRegistry(t)
	repo := newFakeTenantRepo(testTenant("acme"))
	svc := NewApplyPackService(reg, repo)

	slug, _ := value.NewTenantSlug("acme")
	res, err := svc.Apply(context.Background(), slug, "HIPAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PackKey != "HIPAA" {
		t.Fatalf("expected pack HIPAA, got %q", res.PackKey)
	}
	if len(res.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(res.Rules))
	}

	tenant, _ := repo.Get(context.Background(), slug)
	pii := tenant.PIIConfig()
	if !pii.Enabled {
		t.Fatal("expected pii config enabled after apply")
	}
	if len(pii.Rules) != 2 {
		t.Fatalf("expected 2 pii rules, got %d", len(pii.Rules))
	}
	if pii.Rules[0].Action != string(entity.ReactionBlock) {
		t.Fatalf("expected reaction block, got %q", pii.Rules[0].Action)
	}
}

func TestApplyPackService_UnknownPack(t *testing.T) {
	reg := hipaaRegistry(t)
	repo := newFakeTenantRepo(testTenant("acme"))
	svc := NewApplyPackService(reg, repo)

	slug, _ := value.NewTenantSlug("acme")
	_, err := svc.Apply(context.Background(), slug, "NOPE")
	if !errors.Is(err, ErrPackNotFound) {
		t.Fatalf("expected ErrPackNotFound, got %v", err)
	}
}

func TestApplyPackService_MissingTenant(t *testing.T) {
	reg := hipaaRegistry(t)
	repo := newFakeTenantRepo()
	svc := NewApplyPackService(reg, repo)

	slug, _ := value.NewTenantSlug("ghost")
	_, err := svc.Apply(context.Background(), slug, "HIPAA")
	if !errors.Is(err, shielderrors.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestComplianceReportService_ActiveAndDeviated(t *testing.T) {
	reg := hipaaRegistry(t)
	slug, _ := value.NewTenantSlug("acme")

	t.Run("no pack applied reports missing", func(t *testing.T) {
		repo := newFakeTenantRepo(testTenant("acme"))
		svc := NewComplianceReportService(reg, repo)
		rep, err := svc.Report(context.Background(), slug, "HIPAA")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, rr := range rep.Rules {
			if rr.Status != RuleStateMissing {
				t.Fatalf("expected missing for %s, got %s", rr.DetectorType, rr.Status)
			}
		}
	})

	t.Run("customized rule reported deviated", func(t *testing.T) {
		s, _ := value.NewTenantSlug("acme")
		tenant := entity.NewTenant(s, "acme", "X-Auth", []string{"k"}, entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules: []entity.PIARule{
				{Type: string(entity.DetectorTypeRegex), Action: string(entity.ReactionReview)},
				{Type: string(entity.DetectorTypeDictionary), Action: string(entity.ReactionBlock)},
			},
		}))
		repo := newFakeTenantRepo(tenant)
		svc := NewComplianceReportService(reg, repo)

		rep, err := svc.Report(context.Background(), slug, "HIPAA")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		byType := map[entity.DetectorType]RuleState{}
		for _, rr := range rep.Rules {
			byType[rr.DetectorType] = rr.Status
		}
		if byType[entity.DetectorTypeRegex] != RuleStateDeviated {
			t.Fatalf("expected regex deviated (review != block), got %s", byType[entity.DetectorTypeRegex])
		}
		if byType[entity.DetectorTypeDictionary] != RuleStateActive {
			t.Fatalf("expected dictionary active, got %s", byType[entity.DetectorTypeDictionary])
		}
		if len(rep.Deviated()) != 1 || len(rep.Active()) != 1 {
			t.Fatalf("expected 1 deviated + 1 active, got deviated=%d active=%d", len(rep.Deviated()), len(rep.Active()))
		}
	})
}

func TestApplyPackService_ReapplyIsIdempotent(t *testing.T) {
	reg := hipaaRegistry(t)
	repo := newFakeTenantRepo(testTenant("acme"))
	svc := NewApplyPackService(reg, repo)
	slug, _ := value.NewTenantSlug("acme")

	if _, err := svc.Apply(context.Background(), slug, "HIPAA"); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	first, _ := repo.Get(context.Background(), slug)
	firstRules := len(first.PIIConfig().Rules)

	if _, err := svc.Apply(context.Background(), slug, "HIPAA"); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	second, _ := repo.Get(context.Background(), slug)
	if len(second.PIIConfig().Rules) != firstRules {
		t.Fatalf("re-apply duplicated rules: first=%d second=%d", firstRules, len(second.PIIConfig().Rules))
	}
	if second.PIIConfig().Rules[0].Action != string(entity.ReactionBlock) {
		t.Fatalf("re-apply changed rule action: %+v", second.PIIConfig().Rules[0])
	}
}

func TestApplyPackService_NilRegistryRejected(t *testing.T) {
	repo := newFakeTenantRepo(testTenant("acme"))
	svc := NewApplyPackService(nil, repo)
	slug, _ := value.NewTenantSlug("acme")

	_, err := svc.Apply(context.Background(), slug, "HIPAA")
	if !errors.Is(err, ErrPackInvalid) {
		t.Fatalf("expected ErrPackInvalid for nil registry, got %v", err)
	}
	tenant, _ := repo.Get(context.Background(), slug)
	if tenant.PIIConfig().Enabled {
		t.Fatal("expected tenant unchanged after failed apply")
	}
}

func TestComplianceReportService_UnknownPack(t *testing.T) {
	reg := hipaaRegistry(t)
	repo := newFakeTenantRepo(testTenant("acme"))
	svc := NewComplianceReportService(reg, repo)
	slug, _ := value.NewTenantSlug("acme")

	_, err := svc.Report(context.Background(), slug, "NOPE")
	if !errors.Is(err, ErrPackNotFound) {
		t.Fatalf("expected ErrPackNotFound, got %v", err)
	}
}

func TestComplianceReportService_MissingTenant(t *testing.T) {
	reg := hipaaRegistry(t)
	repo := newFakeTenantRepo()
	svc := NewComplianceReportService(reg, repo)
	slug, _ := value.NewTenantSlug("ghost")

	_, err := svc.Report(context.Background(), slug, "HIPAA")
	if !errors.Is(err, shielderrors.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound, got %v", err)
	}
}
