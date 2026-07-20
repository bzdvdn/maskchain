package tenant

// @sk-task 80-tenant-isolation#T1.1: TenantRepository interface (AC-001, AC-003, AC-005)
//
// Repository defines the interface for domain operations.
type Repository interface {
	FindByAPIKey(key string) (*Tenant, bool)
	FindBySlug(slug string) (*Tenant, bool)
	All() []*Tenant
}
