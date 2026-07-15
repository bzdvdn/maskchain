//go:build integration

package sessionrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

func setupValkey(t *testing.T) valkey.Client {
	t.Helper()
	addr := os.Getenv("SHIELD_TEST_VALKEY_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{addr},
	})
	if err != nil {
		t.Fatalf("valkey.NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	ctx := context.Background()
	if err := client.Do(ctx, client.B().Flushall().Build()).Error(); err != nil {
		t.Fatalf("flushall: %v", err)
	}
	return client
}

func newTestSessionForCache(id, tenantID string) *session.Session {
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

// @sk-test sessions#T3.3: TestValkeySessionCache_SaveAndGet (AC-008)
func TestValkeySessionCache_SaveAndGet(t *testing.T) {
	ctx := context.Background()
	client := setupValkey(t)
	cache := NewValkeySessionCache(client, 5*time.Minute)

	sess := newTestSessionForCache("vk-save-1", "tenant-alpha")
	err := cache.Save(ctx, sess)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := cache.Get(ctx, "tenant-alpha", "vk-save-1")
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

// @sk-test sessions#T3.3: TestValkeySessionCache_GetNotFound (AC-008)
func TestValkeySessionCache_GetNotFound(t *testing.T) {
	ctx := context.Background()
	client := setupValkey(t)
	cache := NewValkeySessionCache(client, 5*time.Minute)

	_, err := cache.Get(ctx, "tenant-alpha", "nonexistent")
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// @sk-test sessions#T3.3: TestValkeySessionCache_DeleteExpired (AC-008)
func TestValkeySessionCache_DeleteExpired(t *testing.T) {
	ctx := context.Background()
	client := setupValkey(t)
	cache := NewValkeySessionCache(client, 5*time.Minute)

	sess1 := newTestSessionForCache("vk-del-1", "tenant-alpha")
	sess2 := newTestSessionForCache("vk-del-2", "tenant-alpha")
	if err := cache.Save(ctx, sess1); err != nil {
		t.Fatalf("Save sess1: %v", err)
	}
	if err := cache.Save(ctx, sess2); err != nil {
		t.Fatalf("Save sess2: %v", err)
	}

	deleted, err := cache.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	t.Logf("deleted %d keys", deleted)
}

// @sk-test sessions#T3.3: TestValkeySessionCache_NilClient (AC-008)
func TestValkeySessionCache_NilClient(t *testing.T) {
	cache := NewValkeySessionCache(nil, 5*time.Minute)
	ctx := context.Background()
	sess := newTestSessionForCache("nil-test", "tenant-alpha")

	err := cache.Save(ctx, sess)
	if err != nil {
		t.Errorf("Save with nil client: expected nil, got %v", err)
	}

	_, err = cache.Get(ctx, "t", "nil-test")
	if err != session.ErrSessionNotFound {
		t.Errorf("Get with nil client: expected ErrSessionNotFound, got %v", err)
	}

	deleted, err := cache.DeleteExpired(ctx)
	if err != nil || deleted != 0 {
		t.Errorf("DeleteExpired with nil client: expected 0, nil, got %d, %v", deleted, err)
	}
}
