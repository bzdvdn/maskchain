package dictionary

import (
	"testing"
)

// @sk-test cleanup-profile-repository#T2.3: TestDictionaryValueObject (AC-004)
func TestNewDictionary_Valid(t *testing.T) {
	entries := []interface{}{"secret", "admin"}

	d := NewDictionary("test-dict", entries, MatchModeExact)
	if d.Name() != "test-dict" {
		t.Errorf("expected name 'test-dict', got %q", d.Name())
	}
	if len(d.Entries()) != 2 || d.Entries()[0] != "secret" {
		t.Errorf("unexpected entries: %v", d.Entries())
	}
	vals := d.AllValues()
	if len(vals) != 2 || vals[0] != "secret" {
		t.Errorf("unexpected AllValues: %v", vals)
	}
	if d.MatchMode() != MatchModeExact {
		t.Errorf("expected exact mode, got %v", d.MatchMode())
	}
}

// @sk-test cleanup-profile-repository#T2.3: TestDictionaryNilEntries (AC-004)
func TestNewDictionary_NilEntries(t *testing.T) {
	d := NewDictionary("nil-dict", nil, MatchModeExact)
	if d.Entries() != nil {
		t.Errorf("expected nil entries, got %v", d.Entries())
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
