package shield

import (
	"context"

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

// @sk-task 20-shield-domain#T3.1: Implement IncidentRepository interface (AC-005)
// @sk-task 60-audit-incidents#T1.3: Add List method with filtering and pagination (AC-001, AC-006)
type IncidentRepository interface {
	Save(ctx context.Context, incident *entity.Incident) error
	FindByID(ctx context.Context, id string) (*entity.Incident, error)
	ListByProfile(ctx context.Context, profileID value.ProfileID) ([]*entity.Incident, error)
	ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Incident, error)
	List(ctx context.Context, filter IncidentFilter) ([]*entity.Incident, int, error)
}
