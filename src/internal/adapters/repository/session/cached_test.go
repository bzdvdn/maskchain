//go:build integration

package sessionrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

func setupPGForCache(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("SHIELD_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("SHIELD_TEST_PG_DSN not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `DELETE FROM sessions`); err != nil {
		t.Fatalf("clean sessions: %v", err)
	}
	return pool
}

// @sk-test sessions#T3.3: TestCachedSessionStore_GracefulDegradation (AC-008)
func TestCachedSessionStore_GracefulDegradation(t *testing.T) {
	observedZap, logs := observer.New(zap.WarnLevel)
	log := zap.New(observedZap)

	pool := setupPGForCache(t)
	primary := NewPostgresSessionStore(pool)
	secondary := NewValkeySessionCache(nil, 5*time.Minute)
	store := NewCachedSessionStore(primary, secondary, log)

	ctx := context.Background()
	now := time.Now().UTC()
	sess := &session.Session{
		SessionID: "graceful-degradation-1",
		TenantID:  "tenant-alpha",
		Model:     "gpt-4",
		Status:    session.SessionStatusActive,
		TTL:       30 * time.Minute,
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
	}

	err := store.Save(ctx, sess)
	if err != nil {
		t.Fatalf("Save should succeed with nil Valkey: %v", err)
	}

	got, err := store.Get(ctx, "tenant-alpha", "graceful-degradation-1")
	if err != nil {
		t.Fatalf("Get should succeed with nil Valkey: %v", err)
	}
	if got.SessionID != sess.SessionID {
		t.Errorf("expected %s, got %s", sess.SessionID, got.SessionID)
	}
	if got.TenantID != "tenant-alpha" {
		t.Errorf("expected tenant-alpha, got %s", got.TenantID)
	}

	err = store.IncrementCounts(ctx, "tenant-alpha", "graceful-degradation-1", 50, 1, 2, 1, 1, 0)
	if err != nil {
		t.Fatalf("IncrementCounts should succeed: %v", err)
	}

	err = store.Close(ctx, "tenant-alpha", "graceful-degradation-1")
	if err != nil {
		t.Fatalf("Close should succeed: %v", err)
	}

	got, err = store.Get(ctx, "tenant-alpha", "graceful-degradation-1")
	if err != nil {
		t.Fatalf("Get after close: %v", err)
	}
	if got.Status != session.SessionStatusClosed {
		t.Errorf("expected closed, got %s", got.Status)
	}

	if logs.Len() > 0 {
		t.Logf("WARN logs recorded: %d entries", logs.Len())
		for _, entry := range logs.All() {
			t.Logf("  %s: %s", entry.Level, entry.Message)
		}
	}
}

// @sk-test sessions#T3.3: TestCachedSessionStore_SaveGetRoundTrip (AC-008)
func TestCachedSessionStore_SaveGetRoundTrip(t *testing.T) {
	pool := setupPGForCache(t)
	primary := NewPostgresSessionStore(pool)
	secondary := NewValkeySessionCache(nil, 5*time.Minute)
	log := zap.NewNop()
	store := NewCachedSessionStore(primary, secondary, log)

	ctx := context.Background()
	now := time.Now().UTC()
	sess := &session.Session{
		SessionID: "cached-rt-1",
		TenantID:  "tenant-alpha",
		Model:     "gpt-4",
		Status:    session.SessionStatusActive,
		TTL:       30 * time.Minute,
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
	}

	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, "tenant-alpha", "cached-rt-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SessionID != "cached-rt-1" {
		t.Errorf("expected cached-rt-1, got %s", got.SessionID)
	}
}
