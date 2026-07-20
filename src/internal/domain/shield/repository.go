package shield

import (
	"context"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task tenant-profile-sync#T1.3: Implement TenantRepository interface (AC-001, AC-005, AC-008)
//
// TenantRepository defines the interface for domain operations.
type TenantRepository interface {
	List(ctx context.Context) ([]*entity.Tenant, error)
	Get(ctx context.Context, slug value.TenantSlug) (*entity.Tenant, error)
	Create(ctx context.Context, tenant *entity.Tenant) error
	Update(ctx context.Context, tenant *entity.Tenant) error
	Delete(ctx context.Context, slug value.TenantSlug) error
	GetDictionaries(ctx context.Context, slug value.TenantSlug) ([]*dictionary.Dictionary, error)
	UpdateDictionaries(ctx context.Context, slug value.TenantSlug, dicts []*dictionary.Dictionary) error
}

// @sk-task tenant-profile-sync#T1.4: Implement TenantResolver interface (AC-002, AC-003, AC-004)
//
// TenantResolver defines the interface for domain operations.
type TenantResolver interface {
	List(ctx context.Context) ([]*entity.Tenant, error)
	Get(ctx context.Context, slug value.TenantSlug) (*entity.Tenant, error)
	SyncConfig(ctx context.Context, tenants map[string]*entity.Tenant) error
}
