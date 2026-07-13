package dictionaryrepo

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 102-profile-cache#T3.4: Test PubSubSubscriber handleMessage evicts LRU (AC-005)

func TestPubSubSubscriber_HandleMessage_TracksInvalidation(t *testing.T) {
	tracker := NewInvalidationTracker()
	metrics := newSpyMetrics()

	sub := NewPubSubSubscriber(nil, tracker, metrics, slog.Default())

	sub.handleMessage(valkey.PubSubMessage{
		Pattern: "dictionary.invalidate:*",
		Channel: "dictionary.invalidate:my-profile",
		Message: "",
	})

	if !tracker.CheckAndClear("my-profile") {
		t.Fatal("expected slug to be tracked as invalidated")
	}
	if v := metrics.invalidations["pubsub"]; v != 1 {
		t.Fatalf("expected 1 pubsub invalidation metric, got %d", v)
	}
}

func TestPubSubSubscriber_HandleMessage_UnknownChannel(t *testing.T) {
	tracker := NewInvalidationTracker()
	metrics := newSpyMetrics()

	sub := NewPubSubSubscriber(nil, tracker, metrics, slog.Default())

	sub.handleMessage(valkey.PubSubMessage{
		Channel: "unknown:channel",
		Message: "",
	})

	if tracker.CheckAndClear("test") {
		t.Fatal("expected no tracking for unknown channel")
	}
}

func TestInvalidationTracker_CheckAndClear(t *testing.T) {
	tracker := NewInvalidationTracker()
	tracker.Add("slug-a")
	tracker.Add("slug-b")

	if !tracker.CheckAndClear("slug-a") {
		t.Fatal("expected slug-a to be invalidated")
	}
	if tracker.CheckAndClear("slug-a") {
		t.Fatal("expected slug-a to be cleared after first check")
	}
	if !tracker.CheckAndClear("slug-b") {
		t.Fatal("expected slug-b to be invalidated")
	}
}

func TestCache_FindBySlug_SkipsLRUAfterInvalidation(t *testing.T) {
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	vk.getErr = errors.New("valkey down")
	lru := NewDictionaryLRUCache(100)
	tracker := NewInvalidationTracker()
	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	profile := makeTestProfile("t1", "my-profile", "test")
	lru.Add("t1:my-profile", DictionaryMetadataFromProfile(profile, 1))

	// Mark as invalidated via PubSub
	tracker.Add("my-profile")

	tid, _ := value.NewTenantID("t1")
	slug, _ := value.NewProfileSlug("my-profile")

	cache := NewDictionaryCache(pg, vk, lru, dictLoader, slog.Default(), nil, metrics, tracker)

	result, err := cache.FindBySlug(context.Background(), tid, slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil since profile not in PG and LRU was skipped")
	}
	// Tracker should be cleared after check
	if tracker.CheckAndClear("my-profile") {
		t.Fatal("expected tracker to be cleared after FindBySlug")
	}
	// Should NOT have LRU hit
	if v := metrics.hits["find_by_slug|lru"]; v != 0 {
		t.Fatalf("expected 0 LRU hits (was skipped), got %d", v)
	}
	// Should have stale metric from valkey error
	if v := metrics.stale["find_by_slug"]; v != 1 {
		t.Fatalf("expected 1 stale metric, got %d", v)
	}
}

// @sk-test 102-profile-cache#T3.4: Test Save publishes invalidation (AC-005, AC-007)

func TestDictionaryCache_Save_PublishesInvalidation(t *testing.T) {
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewDictionaryLRUCache(100)
	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	versionFunc := func(ctx context.Context, tenantID, slug string) (int, error) {
		return 1, nil
	}

	cache := NewDictionaryCache(pg, vk, lru, dictLoader, slog.Default(), versionFunc, metrics, nil)

	profile := makeTestProfile("t1", "my-profile", "test")
	if err := cache.Save(context.Background(), profile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vk.publishedSlugs) != 1 {
		t.Fatalf("expected 1 published slug, got %d", len(vk.publishedSlugs))
	}
	if vk.publishedSlugs[0] != "my-profile" {
		t.Fatalf("expected published slug 'my-profile', got %q", vk.publishedSlugs[0])
	}
}

// @sk-test 102-profile-cache#T3.4: Test Delete publishes invalidation (AC-005)

func TestDictionaryCache_Delete_PublishesInvalidation(t *testing.T) {
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewDictionaryLRUCache(100)
	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	profile := makeTestProfile("t1", "my-profile", "test")
	pg.profilesByID[profile.ID().String()] = profile
	pg.profilesBySlug["t1:my-profile"] = profile

	cache := NewDictionaryCache(pg, vk, lru, dictLoader, slog.Default(), nil, metrics, nil)

	if err := cache.Delete(context.Background(), profile.ID()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vk.publishedSlugs) != 1 {
		t.Fatalf("expected 1 published slug, got %d", len(vk.publishedSlugs))
	}
	if vk.publishedSlugs[0] != "my-profile" {
		t.Fatalf("expected published slug 'my-profile', got %q", vk.publishedSlugs[0])
	}
}
