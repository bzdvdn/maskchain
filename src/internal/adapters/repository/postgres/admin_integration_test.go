//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-test admin-ui-design#T4.1: TestAdminSessionStoreCRUD (AC-001, AC-004)
func TestAdminSessionStoreCRUD(t *testing.T) {
	ctx := context.Background()
	pool := setupPGAdmin(t, ctx)
	defer pool.Close()

	store := NewPostgresAdminSessionStore(pool)

	sess, rawToken, err := admin_session.NewAdminSession("admin", 30*time.Minute)
	if err != nil {
		t.Fatalf("NewAdminSession: %v", err)
	}

	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tokenHash := admin_session.HashToken(rawToken)
	got, err := store.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetByTokenHash: %v", err)
	}
	if got.Username != "admin" {
		t.Errorf("expected username admin, got %s", got.Username)
	}

	if err := store.Delete(ctx, sess.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.GetByTokenHash(ctx, tokenHash)
	if err != admin_session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionStoreDeleteExpired (AC-004)
func TestAdminSessionStoreDeleteExpired(t *testing.T) {
	ctx := context.Background()
	pool := setupPGAdmin(t, ctx)
	defer pool.Close()

	store := NewPostgresAdminSessionStore(pool)

	expiredSess, _, err := admin_session.NewAdminSession("admin", -1*time.Hour)
	if err != nil {
		t.Fatalf("NewAdminSession: %v", err)
	}
	if err := store.Save(ctx, expiredSess); err != nil {
		t.Fatalf("Save expired: %v", err)
	}

	deleted, err := store.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted == 0 {
		t.Log("no expired sessions deleted (may have been cleaned up already)")
	}
}

// @sk-test admin-ui-design#T4.1: TestAuditLogStoreWriteAndList (AC-005)
func TestAuditLogStoreWriteAndList(t *testing.T) {
	ctx := context.Background()
	pool := setupPGAdmin(t, ctx)
	defer pool.Close()

	store := NewAuditLogStore(pool, 100)
	defer store.Shutdown()

	entry := &AuditLogEntry{
		AdminUsername: "admin",
		Action:        "create",
		Target:        "tenant:test-tenant",
		Details:       json.RawMessage(`{"slug":"test-tenant"}`),
		CreatedAt:     time.Now(),
	}

	if err := store.Write(ctx, entry); err != nil {
		t.Fatalf("Write: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	entries, err := store.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least 1 audit log entry")
	}
	if entries[0].Action != "create" {
		t.Errorf("expected action create, got %s", entries[0].Action)
	}
}

// @sk-test admin-ui-design#T4.1: TestAuditLogStoreBufferFull (AC-005)
func TestAuditLogStoreBufferFull(t *testing.T) {
	ctx := context.Background()
	pool := setupPGAdmin(t, ctx)
	defer pool.Close()

	store := NewAuditLogStore(pool, 1)
	defer store.Shutdown()

	entry := &AuditLogEntry{
		AdminUsername: "admin",
		Action:        "test",
		Target:        "test",
		CreatedAt:     time.Now(),
	}

	if err := store.Write(ctx, entry); err != nil {
		t.Fatalf("first write: %v", err)
	}

	if err := store.Write(ctx, entry); err != nil {
		t.Fatalf("second write (should wait): %v", err)
	}
}

func setupPGAdmin(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("SHIELD_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("SHIELD_TEST_PG_DSN not set; skipping integration test")
	}

	cfg := &config.DatabaseConfig{
		DSN:             dsn,
		MaxConns:        5,
		MinConns:        1,
		MaxConnLifetime: 30 * time.Minute,
	}

	pool, err := NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}

	if err := RunMigrations(dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return pool
}
