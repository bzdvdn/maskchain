//go:build integration

package sessionrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

func setupPG(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("SHIELD_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("SHIELD_TEST_PG_DSN not set; skipping integration test")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `DELETE FROM sessions`)
	if err != nil {
		t.Fatalf("clean sessions: %v", err)
	}

	return pool
}

func newTestSession(id, tenantID string) *session.Session {
	now := time.Now().UTC()
	return &session.Session{
		SessionID: id,
		TenantID:  tenantID,
		Model:     "gpt-4",
		Status:    session.SessionStatusActive,
		TTL:       30 * time.Minute,
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
	}
}

// @sk-test sessions#T2.4: Integration test for PostgresSessionStore Save and Get (AC-001, AC-003)
func TestPostgresSessionStore_SaveAndGet(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	sess := newTestSession("it-save-1", "tenant-alpha")
	err := store.Save(ctx, sess)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, "tenant-alpha", "it-save-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SessionID != sess.SessionID {
		t.Errorf("expected %s, got %s", sess.SessionID, got.SessionID)
	}
	if got.TenantID != sess.TenantID {
		t.Errorf("expected %s, got %s", sess.TenantID, got.TenantID)
	}
}

// @sk-test sessions#T2.4: Integration test for Save conflict (AC-001)
func TestPostgresSessionStore_SaveConflict(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	sess := newTestSession("it-conflict-1", "tenant-alpha")
	err := store.Save(ctx, sess)
	if err != nil {
		t.Fatalf("first Save: %v", err)
	}

	err = store.Save(ctx, sess)
	if err != session.ErrSessionConflict {
		t.Errorf("expected ErrSessionConflict, got %v", err)
	}
}

// @sk-test sessions#T2.4: Integration test for Get wrong tenant (AC-003)
func TestPostgresSessionStore_GetWrongTenant(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	sess := newTestSession("it-tenant-scope", "tenant-alpha")
	err := store.Save(ctx, sess)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err = store.Get(ctx, "tenant-beta", "it-tenant-scope")
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// @sk-test sessions#T2.4: Integration test for Get nonexistent (AC-001)
func TestPostgresSessionStore_GetNotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	_, err := store.Get(ctx, "tenant-alpha", "nonexistent")
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// @sk-test sessions#T2.4: Integration test for IncrementCounts (AC-002)
func TestPostgresSessionStore_IncrementCounts(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	sess := newTestSession("it-incr-1", "tenant-alpha")
	err := store.Save(ctx, sess)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	err = store.IncrementCounts(ctx, "tenant-alpha", "it-incr-1", 100, 2, 3, 2, 1, 0)
	if err != nil {
		t.Fatalf("IncrementCounts: %v", err)
	}

	got, err := store.Get(ctx, "tenant-alpha", "it-incr-1")
	if err != nil {
		t.Fatalf("Get after increment: %v", err)
	}
	if got.TokenCount != 100 {
		t.Errorf("expected TokenCount=100, got %d", got.TokenCount)
	}
	if got.MessageCount != 2 {
		t.Errorf("expected MessageCount=2, got %d", got.MessageCount)
	}
	if got.DictMaskCount != 2 {
		t.Errorf("expected DictMaskCount=2, got %d", got.DictMaskCount)
	}

	// atomic increment: add again
	err = store.IncrementCounts(ctx, "tenant-alpha", "it-incr-1", 50, 1, 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("IncrementCounts second: %v", err)
	}

	got, err = store.Get(ctx, "tenant-alpha", "it-incr-1")
	if err != nil {
		t.Fatalf("Get after second increment: %v", err)
	}
	if got.TokenCount != 150 {
		t.Errorf("expected TokenCount=150, got %d", got.TokenCount)
	}
	if got.MessageCount != 3 {
		t.Errorf("expected MessageCount=3, got %d", got.MessageCount)
	}
}

// @sk-test sessions#T2.4: Integration test for ExtendTTL (AC-005)
func TestPostgresSessionStore_ExtendTTL(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	sess := newTestSession("it-ext-1", "tenant-alpha")
	err := store.Save(ctx, sess)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	newExpiry := time.Now().UTC().Add(24 * time.Hour)
	err = store.ExtendTTL(ctx, "tenant-alpha", "it-ext-1", newExpiry)
	if err != nil {
		t.Fatalf("ExtendTTL: %v", err)
	}

	got, err := store.Get(ctx, "tenant-alpha", "it-ext-1")
	if err != nil {
		t.Fatalf("Get after extend: %v", err)
	}
	if !got.ExpiresAt.After(sess.ExpiresAt) {
		t.Errorf("expected expires_at to be extended")
	}
}

// @sk-test sessions#T2.4: Integration test for Close (AC-006)
func TestPostgresSessionStore_Close(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	sess := newTestSession("it-close-1", "tenant-alpha")
	err := store.Save(ctx, sess)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	err = store.Close(ctx, "tenant-alpha", "it-close-1")
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := store.Get(ctx, "tenant-alpha", "it-close-1")
	if err != nil {
		t.Fatalf("Get after close: %v", err)
	}
	if got.Status != session.SessionStatusClosed {
		t.Errorf("expected closed, got %s", got.Status)
	}

	// close on already closed returns not found (no active row)
	err = store.Close(ctx, "tenant-alpha", "it-close-1")
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound on double close, got %v", err)
	}
}

// @sk-test sessions#T2.4: Integration test for ListByTenant with pagination (AC-004)
func TestPostgresSessionStore_ListByTenant(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	for i := 0; i < 3; i++ {
		id := "it-list-" + string(rune('a'+i))
		sess := newTestSession(id, "tenant-alpha")
		if err := store.Save(ctx, sess); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}

	t.Run("all items", func(t *testing.T) {
		result, err := store.ListByTenant(ctx, "tenant-alpha", 1, 10)
		if err != nil {
			t.Fatalf("ListByTenant: %v", err)
		}
		if result.Total != 3 {
			t.Errorf("expected Total=3, got %d", result.Total)
		}
		if len(result.Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(result.Items))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		result, err := store.ListByTenant(ctx, "tenant-alpha", 1, 1)
		if err != nil {
			t.Fatalf("ListByTenant: %v", err)
		}
		if result.Total != 3 {
			t.Errorf("expected Total=3, got %d", result.Total)
		}
		if len(result.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(result.Items))
		}
	})

	t.Run("wrong tenant", func(t *testing.T) {
		result, err := store.ListByTenant(ctx, "tenant-beta", 1, 10)
		if err != nil {
			t.Fatalf("ListByTenant: %v", err)
		}
		if result.Total != 0 {
			t.Errorf("expected Total=0, got %d", result.Total)
		}
	})
}

// @sk-test sessions#T2.4: Integration test for DeleteExpired (AC-007)
func TestPostgresSessionStore_DeleteExpired(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	store := NewPostgresSessionStore(pool)

	now := time.Now().UTC()
	expiredSess := &session.Session{
		SessionID: "it-expired-1",
		TenantID:  "tenant-alpha",
		Model:     "gpt-4",
		Status:    session.SessionStatusExpired,
		TTL:       30 * time.Minute,
		CreatedAt: now.Add(-1 * time.Hour),
		ExpiresAt: now.Add(-30 * time.Minute),
	}
	err := store.Save(ctx, expiredSess)
	if err != nil {
		t.Fatalf("Save expired: %v", err)
	}

	activeSess := newTestSession("it-active-1", "tenant-alpha")
	err = store.Save(ctx, activeSess)
	if err != nil {
		t.Fatalf("Save active: %v", err)
	}

	deleted, err := store.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted == 0 {
		t.Errorf("expected at least 1 deleted, got %d", deleted)
	}

	_, err = store.Get(ctx, "tenant-alpha", "it-expired-1")
	if err != session.ErrSessionNotFound {
		t.Errorf("expected expired session to be deleted")
	}

	_, err = store.Get(ctx, "tenant-alpha", "it-active-1")
	if err != nil {
		t.Errorf("active session should remain: %v", err)
	}
}
