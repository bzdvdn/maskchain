package dictionary

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryValueObject (AC-001)
func TestNewDictionary_Valid(t *testing.T) {
	slug, _ := value.NewProfileSlug("test-profile")
	entries := []string{"secret", "admin"}

	d := NewDictionary(slug, "test-dict", entries, MatchModeExact)
	if d.ProfileSlug() != slug {
		t.Errorf("expected slug %v, got %v", slug, d.ProfileSlug())
	}
	if d.Name() != "test-dict" {
		t.Errorf("expected name 'test-dict', got %q", d.Name())
	}
	if len(d.Entries()) != 2 || d.Entries()[0] != "secret" {
		t.Errorf("unexpected entries: %v", d.Entries())
	}
	if d.MatchMode() != MatchModeExact {
		t.Errorf("expected exact mode, got %v", d.MatchMode())
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryNilEntries (AC-001)
func TestNewDictionary_NilEntries(t *testing.T) {
	slug, _ := value.NewProfileSlug("test-profile")
	d := NewDictionary(slug, "nil-dict", nil, MatchModeExact)
	if d.Entries() != nil {
		t.Errorf("expected nil entries, got %v", d.Entries())
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryRepositoryInMemory (AC-002)
func TestDictionaryRepository_SaveAndFind(t *testing.T) {
	ctx := context.Background()
	repo := newInMemoryRepo()

	slug, _ := value.NewProfileSlug("test-profile")
	dict := NewDictionary(slug, "test", []string{"a", "b"}, MatchModeExact)

	if err := repo.Save(ctx, dict); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	found, err := repo.FindByProfileSlug(ctx, slug.String())
	if err != nil {
		t.Fatalf("FindByProfileSlug failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected dictionary, got nil")
	}
	if found.Name() != "test" {
		t.Errorf("expected name 'test', got %q", found.Name())
	}

	if err := repo.Delete(ctx, slug.String()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	missing, err := repo.FindByProfileSlug(ctx, slug.String())
	if err != nil {
		t.Fatalf("FindByProfileSlug after delete failed: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil after delete, got %v", missing)
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestMatchModeValues (AC-001)
func TestMatchMode_Values(t *testing.T) {
	if MatchModeExact.String() != "exact" {
		t.Errorf("expected 'exact', got %q", MatchModeExact.String())
	}
	if MatchModeContains.String() != "contains" {
		t.Errorf("expected 'contains', got %q", MatchModeContains.String())
	}
	if MatchModeRegex.String() != "regex" {
		t.Errorf("expected 'regex', got %q", MatchModeRegex.String())
	}
	if MatchModeFuzzy.String() != "fuzzy" {
		t.Errorf("expected 'fuzzy', got %q", MatchModeFuzzy.String())
	}
}
