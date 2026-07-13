package profilerepo

import (
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 102-profile-cache#T2.6: Test LRU basic operations

func TestProfileLRUCache_AddGet(t *testing.T) {
	lru := NewProfileLRUCache(100)
	meta := &ProfileMetadata{ID: "1", Slug: "test", TenantID: "t1", Name: "test", Version: 1}
	lru.Add("t1:test", meta)

	got, ok := lru.Get("t1:test")
	if !ok {
		t.Fatal("expected to find key")
	}
	if got.Version != 1 {
		t.Fatalf("expected version 1, got %d", got.Version)
	}
}

func TestProfileLRUCache_Remove(t *testing.T) {
	lru := NewProfileLRUCache(100)
	lru.Add("t1:test", &ProfileMetadata{ID: "1", Version: 1})
	lru.Remove("t1:test")

	_, ok := lru.Get("t1:test")
	if ok {
		t.Fatal("expected key to be removed")
	}
}

func TestProfileLRUCache_Eviction(t *testing.T) {
	lru := NewProfileLRUCache(2)
	lru.Add("k1", &ProfileMetadata{ID: "1", Version: 1})
	lru.Add("k2", &ProfileMetadata{ID: "2", Version: 1})
	lru.Add("k3", &ProfileMetadata{ID: "3", Version: 1}) // should evict k1

	if lru.Len() > 2 {
		t.Fatalf("expected len <= 2, got %d", lru.Len())
	}
}

func TestProfileMetadataFromProfile(t *testing.T) {
	tid, _ := value.NewTenantID("t1")
	slug, _ := value.NewProfileSlug("test")
	pid, _ := value.NewProfileID("p1")

	profile := entity.NewProfile(pid, slug, tid, "test-name")
	meta := ProfileMetadataFromProfile(profile, 5)

	if meta.ID != "p1" {
		t.Fatalf("expected id p1, got %q", meta.ID)
	}
	if meta.Slug != "test" {
		t.Fatalf("expected slug test, got %q", meta.Slug)
	}
	if meta.Version != 5 {
		t.Fatalf("expected version 5, got %d", meta.Version)
	}
	if meta.Name != "test-name" {
		t.Fatalf("expected name test-name, got %q", meta.Name)
	}
	if !meta.Enabled {
		t.Fatal("expected enabled true")
	}
}
