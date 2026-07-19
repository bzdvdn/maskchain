package resolver

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type mockTenantRepo struct {
	listFn func() ([]*entity.Tenant, error)
	getFn  func(ctx context.Context, slug value.TenantSlug) (*entity.Tenant, error)
}

func (m *mockTenantRepo) List(ctx context.Context) ([]*entity.Tenant, error) {
	return m.listFn()
}
func (m *mockTenantRepo) Get(ctx context.Context, slug value.TenantSlug) (*entity.Tenant, error) {
	return m.getFn(ctx, slug)
}
func (m *mockTenantRepo) Create(ctx context.Context, tenant *entity.Tenant) error {
	return nil
}
func (m *mockTenantRepo) Update(ctx context.Context, tenant *entity.Tenant) error {
	return nil
}
func (m *mockTenantRepo) Delete(ctx context.Context, slug value.TenantSlug) error {
	return nil
}
func (m *mockTenantRepo) GetDictionaries(ctx context.Context, slug value.TenantSlug) ([]*dictionary.Dictionary, error) {
	return nil, nil
}
func (m *mockTenantRepo) UpdateDictionaries(ctx context.Context, slug value.TenantSlug, dicts []*dictionary.Dictionary) error {
	return nil
}

// @sk-test release-readiness: DBFirstTenantResolver unit tests
func TestNewDBFirstTenantResolver(t *testing.T) {
	repo := &mockTenantRepo{}
	r := NewDBFirstTenantResolver(repo, nil)
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
}

// @sk-test release-readiness: DBFirstTenantResolver unit tests
func TestDBFirstTenantResolver_List(t *testing.T) {
	ctx := context.Background()
	repo := &mockTenantRepo{
		listFn: func() ([]*entity.Tenant, error) {
			return []*entity.Tenant{}, nil
		},
	}
	r := NewDBFirstTenantResolver(repo, nil)

	tenants, err := r.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tenants) != 0 {
		t.Errorf("expected 0 tenants, got %d", len(tenants))
	}
}

// @sk-test release-readiness: DBFirstTenantResolver unit tests
func TestDBFirstTenantResolver_Get_FromRepo(t *testing.T) {
	ctx := context.Background()
	slug, _ := value.NewTenantSlug("tenant-alpha")

	repo := &mockTenantRepo{
		getFn: func(_ context.Context, s value.TenantSlug) (*entity.Tenant, error) {
			return entity.NewTenant(s, "Alpha", "Bearer x", nil), nil
		},
	}
	r := NewDBFirstTenantResolver(repo, nil)

	tenant, err := r.Get(ctx, slug)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tenant == nil {
		t.Fatal("expected non-nil tenant")
	}
	if tenant.Name() != "Alpha" {
		t.Errorf("Name = %q, want %q", tenant.Name(), "Alpha")
	}
}

// @sk-test release-readiness: DBFirstTenantResolver unit tests
func TestDBFirstTenantResolver_Get_FallbackToConfig(t *testing.T) {
	ctx := context.Background()
	slug, _ := value.NewTenantSlug("cfg-tenant")

	repo := &mockTenantRepo{
		getFn: func(_ context.Context, s value.TenantSlug) (*entity.Tenant, error) {
			return nil, nil
		},
	}
	cfgTenant := entity.NewTenant(slug, "Config Tenant", "Bearer cfg", nil)
	r := NewDBFirstTenantResolver(repo, map[string]*entity.Tenant{"cfg-tenant": cfgTenant})

	tenant, err := r.Get(ctx, slug)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tenant == nil {
		t.Fatal("expected tenant from config fallback")
	}
	if tenant.Name() != "Config Tenant" {
		t.Errorf("Name = %q, want %q", tenant.Name(), "Config Tenant")
	}
}

// @sk-test release-readiness: DBFirstTenantResolver unit tests
func TestDBFirstTenantResolver_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	slug, _ := value.NewTenantSlug("nonexistent")

	repo := &mockTenantRepo{
		getFn: func(_ context.Context, s value.TenantSlug) (*entity.Tenant, error) {
			return nil, nil
		},
	}
	r := NewDBFirstTenantResolver(repo, nil)

	tenant, err := r.Get(ctx, slug)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tenant != nil {
		t.Fatal("expected nil tenant for unknown slug")
	}
}
