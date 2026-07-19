package sessionrepo

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

// @sk-test release-readiness: session store nil-client/pool unit tests

func TestPostgresSessionStore_NilPool(t *testing.T) {
	store := NewPostgresSessionStore(nil)
	ctx := context.Background()

	_, err := store.Get(ctx, "t", "x")
	if err != session.ErrSessionNotFound {
		t.Errorf("Get with nil pool: expected ErrSessionNotFound, got %v", err)
	}

	err = store.Save(ctx, &session.Session{})
	if err != session.ErrSessionNotFound {
		t.Errorf("Save with nil pool: expected ErrSessionNotFound, got %v", err)
	}

	err = store.IncrementCounts(ctx, "t", "x", 0, 0, 0, 0, 0, 0)
	if err != session.ErrSessionNotFound {
		t.Errorf("IncrementCounts with nil pool: expected ErrSessionNotFound, got %v", err)
	}

	err = store.ExtendTTL(ctx, "t", "x", session.Session{}.ExpiresAt)
	if err != session.ErrSessionNotFound {
		t.Errorf("ExtendTTL with nil pool: expected ErrSessionNotFound, got %v", err)
	}

	err = store.Close(ctx, "t", "x")
	if err != session.ErrSessionNotFound {
		t.Errorf("Close with nil pool: expected ErrSessionNotFound, got %v", err)
	}

	deleted, err := store.DeleteExpired(ctx)
	if err != nil {
		t.Errorf("DeleteExpired with nil pool: expected nil, got %v", err)
	}
	if deleted != 0 {
		t.Errorf("DeleteExpired with nil pool: expected 0, got %d", deleted)
	}

	result, err := store.ListByTenant(ctx, "t", 1, 10)
	if err != nil {
		t.Errorf("ListByTenant with nil pool: expected nil, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListResult")
	}
	if result.Total != 0 {
		t.Errorf("ListByTenant Total: expected 0, got %d", result.Total)
	}

	result, err = store.ListAll(ctx, 1, 10)
	if err != nil {
		t.Errorf("ListAll with nil pool: expected nil, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListResult")
	}
	if result.Total != 0 {
		t.Errorf("ListAll Total: expected 0, got %d", result.Total)
	}
}

func TestValkeySessionCache_NilClient(t *testing.T) {
	cache := NewValkeySessionCache(nil, 0)
	ctx := context.Background()

	sess := &session.Session{SessionID: "nil-test"}
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

	err = cache.IncrementCounts(ctx, "t", "x", 0, 0, 0, 0, 0, 0)
	if err != session.ErrSessionNotFound {
		t.Errorf("IncrementCounts: expected ErrSessionNotFound, got %v", err)
	}

	err = cache.ExtendTTL(ctx, "t", "x", session.Session{}.ExpiresAt)
	if err != session.ErrSessionNotFound {
		t.Errorf("ExtendTTL: expected ErrSessionNotFound, got %v", err)
	}

	err = cache.Close(ctx, "t", "x")
	if err != session.ErrSessionNotFound {
		t.Errorf("Close: expected ErrSessionNotFound, got %v", err)
	}

	result, err := cache.ListByTenant(ctx, "t", 1, 10)
	if err != nil {
		t.Errorf("ListByTenant: expected nil, got %v", err)
	}
	if result == nil || result.Total != 0 {
		t.Errorf("ListByTenant: expected empty result, got %+v", result)
	}

	result, err = cache.ListAll(ctx, 1, 10)
	if err != nil {
		t.Errorf("ListAll: expected nil, got %v", err)
	}
	if result == nil || result.Total != 0 {
		t.Errorf("ListAll: expected empty result, got %+v", result)
	}
}
