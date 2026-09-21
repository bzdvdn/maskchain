package middleware

import (
	"context"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// @sk-task 300-virtual-keys#T2.3: fakeVirtualKeyRepo for middleware tests (AC-001)
type fakeVirtualKeyRepo struct {
	keys []*virtualkey.VirtualKey
}

func (f *fakeVirtualKeyRepo) FindByKeyHash(_ context.Context, hash string) (*virtualkey.VirtualKey, error) {
	for _, k := range f.keys {
		if k.KeyHash == hash && k.Enabled {
			return k, nil
		}
	}
	return nil, virtualkey.ErrNotFound
}

func (f *fakeVirtualKeyRepo) GetById(_ context.Context, id string) (*virtualkey.VirtualKey, error) {
	for _, k := range f.keys {
		if k.ID == id {
			return k, nil
		}
	}
	return nil, virtualkey.ErrNotFound
}

func (f *fakeVirtualKeyRepo) ListByTenant(_ context.Context, tenantID string) ([]*virtualkey.VirtualKey, error) {
	var out []*virtualkey.VirtualKey
	for _, k := range f.keys {
		if k.TenantID == tenantID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeVirtualKeyRepo) List(_ context.Context) ([]*virtualkey.VirtualKey, error) {
	return f.keys, nil
}

// @sk-task ui-production-readiness#T7.1: DB-level pagination/search (AC-008)
func (f *fakeVirtualKeyRepo) ListPaged(_ context.Context, limit, offset int, _ string) ([]*virtualkey.VirtualKey, int, error) {
	total := len(f.keys)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return f.keys[start:end], total, nil
}

func (f *fakeVirtualKeyRepo) Create(_ context.Context, k *virtualkey.VirtualKey) error {
	f.keys = append(f.keys, k)
	return nil
}

func (f *fakeVirtualKeyRepo) Update(_ context.Context, k *virtualkey.VirtualKey) error {
	for i, item := range f.keys {
		if item.ID == k.ID {
			f.keys[i] = k
			return nil
		}
	}
	return virtualkey.ErrNotFound
}

func (f *fakeVirtualKeyRepo) Delete(_ context.Context, id string) error {
	for i, k := range f.keys {
		if k.ID == id {
			f.keys[i].Enabled = false
			return nil
		}
	}
	return virtualkey.ErrNotFound
}

func (f *fakeVirtualKeyRepo) BackfillFromLegacy(_ context.Context, _ map[string][]string) (int, error) {
	return 0, nil
}

// fakeKey builds a virtual key with the given hash for a tenant with slug.
func fakeKey(hash, tenantID string, allowed, blocked []string) *virtualkey.VirtualKey {
	now := time.Now().UTC()
	return &virtualkey.VirtualKey{
		ID:            "id-" + hash,
		TenantID:      tenantID,
		KeyHash:       hash,
		Label:         "test",
		AllowedModels: allowed,
		BlockedModels: blocked,
		Metadata:      map[string]string{},
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
