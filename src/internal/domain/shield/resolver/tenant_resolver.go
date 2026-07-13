package resolver

import (
	"context"
	"fmt"
	"log"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task tenant-profile-sync#T1.4: Implement DBFirstTenantResolver (AC-002, AC-003, AC-004)
type DBFirstTenantResolver struct {
	repo      shield.TenantRepository
	cfgTenants map[string]*entity.Tenant
}

func NewDBFirstTenantResolver(repo shield.TenantRepository, cfgTenants map[string]*entity.Tenant) *DBFirstTenantResolver {
	return &DBFirstTenantResolver{
		repo:       repo,
		cfgTenants: cfgTenants,
	}
}

func (r *DBFirstTenantResolver) List(ctx context.Context) ([]*entity.Tenant, error) {
	tenants, err := r.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolver list: %w", err)
	}
	return tenants, nil
}

func (r *DBFirstTenantResolver) Get(ctx context.Context, slug value.TenantSlug) (*entity.Tenant, error) {
	tenant, err := r.repo.Get(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("resolver get: %w", err)
	}
	if tenant != nil {
		return tenant, nil
	}

	if r.cfgTenants != nil {
		if t, ok := r.cfgTenants[slug.String()]; ok {
			return t, nil
		}
	}

	return nil, nil
}

func (r *DBFirstTenantResolver) SyncConfig(ctx context.Context, tenants map[string]*entity.Tenant) error {
	for slugStr, tenant := range tenants {
		slug, err := value.NewTenantSlug(slugStr)
		if err != nil {
			return fmt.Errorf("invalid config tenant slug %q: %w", slugStr, err)
		}

		existing, err := r.repo.Get(ctx, slug)
		if err != nil {
			return fmt.Errorf("sync config: get tenant %s: %w", slugStr, err)
		}
		if existing != nil {
			continue
		}

		if err := r.repo.Create(ctx, tenant); err != nil {
			return fmt.Errorf("sync config: create tenant %s: %w", slugStr, err)
		}
		log.Printf("[resolver] seeded tenant %q from config", slugStr)
	}
	return nil
}
