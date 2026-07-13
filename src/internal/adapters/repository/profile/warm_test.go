package profilerepo

import (
	"context"
	"log/slog"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 102-profile-cache#T2.6: Test ProfileCacheWarmer warmOne and WarmTenant (AC-010)

func TestCacheWarmer_WarmOne_PopulatesValkeyAndLRU(t *testing.T) {
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewProfileLRUCache(100)

	profile := makeTestProfile("t1", "my-profile", "test")
	pg.profilesBySlug["t1:my-profile"] = profile

	versionFunc := func(ctx context.Context, tenantID, slug string) (int, error) {
		return 1, nil
	}

	warmer := NewProfileCacheWarmer(pg, vk, lru, slog.Default(), versionFunc, 5)

	ref := &profileRef{slug: "my-profile", tenantID: "t1"}
	if err := warmer.warmOne(context.Background(), ref); err != nil {
		t.Fatalf("warmOne failed: %v", err)
	}

	// Valkey populated
	_, vkExists := vk.store["t1:my-profile"]
	if !vkExists {
		t.Fatal("expected Valkey to be populated after warm")
	}
	// LRU populated
	_, lruOK := lru.Get("t1:my-profile")
	if !lruOK {
		t.Fatal("expected LRU to be populated after warm")
	}
}

func TestCacheWarmer_WarmOne_ValkeyHit_NoPG(t *testing.T) {
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewProfileLRUCache(100)

	profile := makeTestProfile("t1", "my-profile", "test")
	val := profileToCacheValue(profile, 2)
	vk.store["t1:my-profile"] = val

	warmer := NewProfileCacheWarmer(pg, vk, lru, slog.Default(), nil, 5)

	ref := &profileRef{slug: "my-profile", tenantID: "t1"}
	if err := warmer.warmOne(context.Background(), ref); err != nil {
		t.Fatalf("warmOne failed: %v", err)
	}

	// LRU populated from Valkey (no PG call)
	meta, lruOK := lru.Get("t1:my-profile")
	if !lruOK {
		t.Fatal("expected LRU to be populated")
	}
	if meta.Version != 2 {
		t.Fatalf("expected version 2 in LRU, got %d", meta.Version)
	}
	if len(pg.profilesBySlug) != 0 {
		t.Fatal("expected no PG call during warm from Valkey")
	}
}

func TestCacheWarmer_WarmTenant(t *testing.T) {
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewProfileLRUCache(100)

	p1 := makeTestProfile("t1", "profile-a", "A")
	p2 := makeTestProfile("t1", "profile-b", "B")
	tid, _ := value.NewTenantID("t1")
	pg.profilesBySlug["t1:profile-a"] = p1
	pg.profilesBySlug["t1:profile-b"] = p2

	versionFunc := func(ctx context.Context, tenantID, slug string) (int, error) {
		return 1, nil
	}

	warmer := NewProfileCacheWarmer(pg, vk, lru, slog.Default(), versionFunc, 2)
	warmer.WarmTenant(context.Background(), tid)

	// Both profiles should be in Valkey and LRU
	_, aVK := vk.store["t1:profile-a"]
	_, bVK := vk.store["t1:profile-b"]
	if !aVK || !bVK {
		t.Fatal("expected both profiles in Valkey after warm")
	}
	_, aLRU := lru.Get("t1:profile-a")
	_, bLRU := lru.Get("t1:profile-b")
	if !aLRU || !bLRU {
		t.Fatal("expected both profiles in LRU after warm")
	}
}
