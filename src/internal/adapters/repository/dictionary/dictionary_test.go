package dictionaryrepo

import (
	"context"
	"testing"
	"time"
)

// @sk-test release-readiness: dictionary Valkey cache unit tests
func TestValkeyDictionaryCache_New(t *testing.T) {
	c := NewValkeyDictionaryCache(nil, 5*time.Minute)
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

// @sk-test release-readiness: dictionary Valkey cache unit tests
func TestValkeyDictionaryCache_Get_NilClient(t *testing.T) {
	c := NewValkeyDictionaryCache(nil, 5*time.Minute)
	ctx := context.Background()

	dicts, err := c.Get(ctx, "test-slug")
	if err != nil {
		t.Fatalf("Get with nil client: %v", err)
	}
	if dicts != nil {
		t.Errorf("expected nil dicts, got %v", dicts)
	}
}

// @sk-test release-readiness: dictionary Valkey cache unit tests
func TestValkeyDictionaryCache_Set_NilClient(t *testing.T) {
	c := NewValkeyDictionaryCache(nil, 5*time.Minute)
	ctx := context.Background()

	err := c.Set(ctx, "test-slug", nil)
	if err != nil {
		t.Fatalf("Set with nil client: %v", err)
	}
}

// @sk-test release-readiness: dictionary Valkey cache unit tests
func TestValkeyDictionaryCache_Delete_NilClient(t *testing.T) {
	c := NewValkeyDictionaryCache(nil, 5*time.Minute)
	ctx := context.Background()

	err := c.Delete(ctx, "test-slug")
	if err != nil {
		t.Fatalf("Delete with nil client: %v", err)
	}
}

// @sk-test release-readiness: dictionary Valkey cache unit tests
func TestValkeyDictionaryCache_Key(t *testing.T) {
	c := NewValkeyDictionaryCache(nil, 5*time.Minute)
	got := c.key("my-slug")
	want := "dict:my-slug"
	if got != want {
		t.Errorf("key() = %q, want %q", got, want)
	}
}
