package virtualkey

import "context"

// @sk-task 300-virtual-keys#T1.2: VirtualKeyRepository port for virtual key persistence (AC-001)
type VirtualKeyRepository interface {
	// FindByKeyHash resolves a key by its SHA-256 digest.
	FindByKeyHash(ctx context.Context, hash string) (*VirtualKey, error)
	// GetById returns a key by its public id.
	GetById(ctx context.Context, id string) (*VirtualKey, error)
	// ListByTenant returns all keys of a tenant.
	ListByTenant(ctx context.Context, tenantID string) ([]*VirtualKey, error)
	// List returns all keys.
	List(ctx context.Context) ([]*VirtualKey, error)
	// Create persists a new virtual key.
	Create(ctx context.Context, key *VirtualKey) error
	// Update persists changes to an existing key.
	Update(ctx context.Context, key *VirtualKey) error
	// Delete soft-deletes a key (revokes it).
	Delete(ctx context.Context, id string) error
	// BackfillFromLegacy creates virtual keys for raw tenant keys that have no
	// matching virtual key yet. It is idempotent.
	BackfillFromLegacy(ctx context.Context, legacy map[string][]string) (int, error)
}
