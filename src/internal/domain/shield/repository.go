package shield

import (
	"context"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T3.1: Implement ProfileRepository interface (AC-005)
type ProfileRepository interface {
	Save(ctx context.Context, profile *entity.Profile) error
	FindByID(ctx context.Context, id value.ProfileID) (*entity.Profile, error)
	FindBySlug(ctx context.Context, tenantID value.TenantID, slug value.ProfileSlug) (*entity.Profile, error)
	ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Profile, error)
	Delete(ctx context.Context, id value.ProfileID) error
}

// @sk-task 60-audit-incidents#T1.3: IncidentFilter struct for list with filtering and pagination (AC-001, AC-006)
type IncidentFilter struct {
	Severity    *string
	Tenant      *string
	ProfileSlug *string
	Page        int
	PageSize    int
}

// @sk-task tenant-profile-sync#T1.3: Implement TenantRepository interface (AC-001, AC-005, AC-008)
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
type TenantResolver interface {
	List(ctx context.Context) ([]*entity.Tenant, error)
	Get(ctx context.Context, slug value.TenantSlug) (*entity.Tenant, error)
	SyncConfig(ctx context.Context, tenants map[string]*entity.Tenant) error
}

// @sk-task 20-shield-domain#T3.1: Implement IncidentRepository interface (AC-005)
// @sk-task 60-audit-incidents#T1.3: Add List method with filtering and pagination (AC-001, AC-006)
type IncidentRepository interface {
	Save(ctx context.Context, incident *entity.Incident) error
	FindByID(ctx context.Context, id string) (*entity.Incident, error)
	ListByProfile(ctx context.Context, profileID value.ProfileID) ([]*entity.Incident, error)
	ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Incident, error)
	List(ctx context.Context, filter IncidentFilter) ([]*entity.Incident, int, error)
}
