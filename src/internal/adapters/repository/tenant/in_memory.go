package tenantrepo

import (
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/tenant"
)

// @sk-task 80-tenant-isolation#T1.3: In-memory TenantRepository with reverse index (AC-001, AC-003)
//
// InMemoryRepository represents a domain entity or configuration.
type InMemoryRepository struct {
	bySlug map[string]*tenant.Tenant
	byKey  map[string]*tenant.Tenant
}

func NewInMemoryRepository(tenants []*tenant.Tenant) (*InMemoryRepository, error) {
	r := &InMemoryRepository{
		bySlug: make(map[string]*tenant.Tenant, len(tenants)),
		byKey:  make(map[string]*tenant.Tenant),
	}
	for _, t := range tenants {
		if _, exists := r.bySlug[t.Slug()]; exists {
			return nil, fmt.Errorf("duplicate tenant slug: %s", t.Slug())
		}
		r.bySlug[t.Slug()] = t
		for _, k := range t.APIKeys() {
			key := k.String()
			if _, exists := r.byKey[key]; exists {
				return nil, fmt.Errorf("duplicate api key for tenant %s: %s", t.Slug(), key)
			}
			r.byKey[key] = t
		}
	}
	return r, nil
}

func (r *InMemoryRepository) FindByAPIKey(key string) (*tenant.Tenant, bool) {
	t, ok := r.byKey[key]
	return t, ok
}

func (r *InMemoryRepository) FindBySlug(slug string) (*tenant.Tenant, bool) {
	t, ok := r.bySlug[slug]
	return t, ok
}

func (r *InMemoryRepository) All() []*tenant.Tenant {
	result := make([]*tenant.Tenant, 0, len(r.bySlug))
	for _, t := range r.bySlug {
		result = append(result, t)
	}
	return result
}
