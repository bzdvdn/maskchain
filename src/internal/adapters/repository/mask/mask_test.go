package maskrepo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-test release-readiness: mask repository unit tests

// --- In-memory mock for MaskStorage ---

type inMemoryMaskStore struct {
	mu   sync.Mutex
	data map[string]*mask.MaskEntry
}

func (s *inMemoryMaskStore) Save(_ context.Context, entry *mask.MaskEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]*mask.MaskEntry)
	}
	if _, exists := s.data[entry.MaskID]; exists {
		return mask.ErrMaskIDConflict
	}
	s.data[entry.MaskID] = entry
	return nil
}

func (s *inMemoryMaskStore) Get(_ context.Context, maskID string) (*mask.MaskEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.data[maskID]
	if !ok {
		return nil, mask.ErrMaskNotFound
	}
	return entry, nil
}

func (s *inMemoryMaskStore) Delete(_ context.Context, maskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, maskID)
	return nil
}

func newTestMaskEntry(id string) *mask.MaskEntry {
	return &mask.MaskEntry{
		MaskID:         id,
		DocumentMaskID: "doc-" + id,
		Replacements:   map[string]string{"key": "value"},
		CreatedAt:      time.Now().UTC(),
	}
}

// --- PostgresMaskRepo with nil pool ---

func TestPostgresMaskRepo_NilPool(t *testing.T) {
	ctx := context.Background()
	repo := NewPostgresMaskRepo(nil)

	_, err := repo.Get(ctx, "test-id")
	if !errors.Is(err, mask.ErrMaskNotFound) {
		t.Errorf("Get with nil pool: expected ErrMaskNotFound, got %v", err)
	}

	err = repo.Save(ctx, newTestMaskEntry("test-id"))
	if !errors.Is(err, mask.ErrMaskNotFound) {
		t.Errorf("Save with nil pool: expected ErrMaskNotFound, got %v", err)
	}

	err = repo.Delete(ctx, "test-id")
	if err != nil {
		t.Errorf("Delete with nil pool: expected nil, got %v", err)
	}
}

// --- ValkeyMaskRepo with nil client ---

func TestValkeyMaskRepo_NilClient(t *testing.T) {
	ctx := context.Background()
	repo := NewValkeyMaskRepo(nil, 5*time.Minute)

	err := repo.Save(ctx, newTestMaskEntry("test-id"))
	if err != nil {
		t.Errorf("Save with nil client: expected nil, got %v", err)
	}

	_, err = repo.Get(ctx, "test-id")
	if !errors.Is(err, mask.ErrMaskNotFound) {
		t.Errorf("Get with nil client: expected ErrMaskNotFound, got %v", err)
	}

	err = repo.Delete(ctx, "test-id")
	if err != nil {
		t.Errorf("Delete with nil client: expected nil, got %v", err)
	}
}

func TestValkeyMaskRepo_Key(t *testing.T) {
	repo := NewValkeyMaskRepo(nil, 0)
	got := repo.key("my-id")
	want := "mask:my-id"
	if got != want {
		t.Errorf("key() = %q, want %q", got, want)
	}
}

// --- CachedMaskRepo with in-memory stores ---

func TestCachedMaskRepo_SaveAndGet(t *testing.T) {
	ctx := context.Background()
	primary := &inMemoryMaskStore{}
	secondary := &inMemoryMaskStore{}
	repo := NewCachedMaskRepo(primary, secondary)

	entry := newTestMaskEntry("cached-1")
	err := repo.Save(ctx, entry)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.Get(ctx, "cached-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.MaskID != "cached-1" {
		t.Errorf("MaskID = %q, want %q", got.MaskID, "cached-1")
	}
	if got.DocumentMaskID != "doc-cached-1" {
		t.Errorf("DocumentMaskID = %q, want %q", got.DocumentMaskID, "doc-cached-1")
	}
	if v, ok := got.Replacements["key"]; !ok || v != "value" {
		t.Errorf("Replacements[\"key\"] = %q, want %q", v, "value")
	}
}

func TestCachedMaskRepo_Delete(t *testing.T) {
	ctx := context.Background()
	primary := &inMemoryMaskStore{}
	secondary := &inMemoryMaskStore{}
	repo := NewCachedMaskRepo(primary, secondary)

	entry := newTestMaskEntry("del-1")
	if err := repo.Save(ctx, entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	err := repo.Delete(ctx, "del-1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = primary.Get(ctx, "del-1")
	if !errors.Is(err, mask.ErrMaskNotFound) {
		t.Errorf("expected entry removed from primary, got %v", err)
	}
}

func TestCachedMaskRepo_Get_MissThenBackfill(t *testing.T) {
	ctx := context.Background()
	primary := &inMemoryMaskStore{}
	secondary := &inMemoryMaskStore{}
	repo := NewCachedMaskRepo(primary, secondary)

	entry := newTestMaskEntry("backfill-1")
	if err := primary.Save(ctx, entry); err != nil {
		t.Fatalf("primary.Save: %v", err)
	}

	got, err := repo.Get(ctx, "backfill-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.MaskID != "backfill-1" {
		t.Errorf("MaskID = %q, want %q", got.MaskID, "backfill-1")
	}

	cached, err := secondary.Get(ctx, "backfill-1")
	if err != nil {
		t.Fatalf("secondary.Get after backfill: %v", err)
	}
	if cached == nil {
		t.Fatal("expected secondary to have backfilled entry")
	}
}

func TestCachedMaskRepo_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewCachedMaskRepo(&inMemoryMaskStore{}, &inMemoryMaskStore{})

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, mask.ErrMaskNotFound) {
		t.Errorf("expected ErrMaskNotFound, got %v", err)
	}
}
