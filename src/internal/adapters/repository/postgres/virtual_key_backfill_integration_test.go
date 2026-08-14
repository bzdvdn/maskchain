//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// @sk-test 403-key-at-rest-encryption#T5.1: TestPgVirtualKeyBackfillIdempotent (AC-005)
func TestPgVirtualKeyBackfillIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	if _, err := pool.Exec(ctx, `DELETE FROM virtual_keys`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (slug, name, auth_header) VALUES
		('acme', 'Acme', 'Authorization'),
		('beta', 'Beta', 'X-Auth')
		ON CONFLICT (slug) DO NOTHING`); err != nil {
		t.Fatalf("seed tenants: %v", err)
	}

	repo := NewPostgresVirtualKeyRepository(pool)
	legacy := map[string][]string{
		"acme": {"sk-legacy-acme-1", "sk-legacy-acme-2"},
		"beta": {"mk-legacy-beta-1"},
	}

	first, err := repo.BackfillFromLegacy(ctx, legacy)
	if err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if first != 3 {
		t.Errorf("expected 3 keys created on first backfill, got %d", first)
	}

	second, err := repo.BackfillFromLegacy(ctx, legacy)
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if second != 0 {
		t.Errorf("expected 0 new keys on idempotent second backfill, got %d", second)
	}

	keys, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("expected exactly 3 virtual keys, got %d", len(keys))
	}

	// The seeded raw keys must be resolvable through the hash.
	for _, raw := range []string{"sk-legacy-acme-1", "mk-legacy-beta-1"} {
		k, err := repo.FindByKeyHash(ctx, virtualkey.KeyHash(raw))
		if err != nil {
			t.Fatalf("resolve backfilled key %q: %v", raw, err)
		}
		if !k.Enabled {
			t.Errorf("backfilled key %q must be enabled", raw)
		}
	}
}