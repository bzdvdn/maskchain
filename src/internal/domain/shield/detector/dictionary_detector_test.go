package detector

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

func testDict(t *testing.T, entries []string, mode dictionary.MatchMode) *dictionary.Dictionary {
	t.Helper()
	slug, _ := value.NewProfileSlug("test-profile")
	return dictionary.NewDictionary(slug, "test", entries, mode)
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryDetectorExact (AC-003)
func TestDictionaryDetector_Exact(t *testing.T) {
	ctx := context.Background()
	dict := testDict(t, []string{"secret", "admin"}, dictionary.MatchModeExact)
	d := NewDictionaryDetector(dict)
	results, err := d.Scan(ctx, "the admin password is secret")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
	if results[0].Fragment != "admin" {
		t.Errorf("expected 'admin', got %q", results[0].Fragment)
	}
	if results[1].Fragment != "secret" {
		t.Errorf("expected 'secret', got %q", results[1].Fragment)
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryDetectorExactNoMatch (AC-003)
func TestDictionaryDetector_ExactNoMatch(t *testing.T) {
	ctx := context.Background()
	dict := testDict(t, []string{"secret"}, dictionary.MatchModeExact)
	d := NewDictionaryDetector(dict)
	results, err := d.Scan(ctx, "hello world")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryDetectorNilDict (AC-003)
func TestDictionaryDetector_NilDict(t *testing.T) {
	ctx := context.Background()
	d := NewDictionaryDetector(nil)
	results, err := d.Scan(ctx, "anything")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil dict, got %d", len(results))
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryDetectorContains (AC-004)
func TestDictionaryDetector_Contains(t *testing.T) {
	ctx := context.Background()
	dict := testDict(t, []string{"example.com", "test"}, dictionary.MatchModeContains)
	d := NewDictionaryDetector(dict)
	results, err := d.Scan(ctx, "visit sub.example.com for test")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryDetectorRegex (AC-005)
func TestDictionaryDetector_Regex(t *testing.T) {
	ctx := context.Background()
	dict := testDict(t, []string{"admin.*", "\\d{3}"}, dictionary.MatchModeRegex)
	d := NewDictionaryDetector(dict)
	results, err := d.Scan(ctx, "admin_test 123")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryDetectorRegexInvalid (AC-005)
func TestDictionaryDetector_RegexInvalid(t *testing.T) {
	ctx := context.Background()
	dict := testDict(t, []string{"[invalid", "valid"}, dictionary.MatchModeRegex)
	d := NewDictionaryDetector(dict)
	results, err := d.Scan(ctx, "this is valid")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only 'valid'), got %d: %v", len(results), results)
	}
	if results[0].Fragment != "valid" {
		t.Errorf("expected 'valid', got %q", results[0].Fragment)
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryDetectorRegistry (AC-007)
func TestDictionaryDetector_Registry(t *testing.T) {
	registry := NewDetectorRegistry()
	dict := testDict(t, []string{"test"}, dictionary.MatchModeExact)
	det := NewDictionaryDetector(dict)

	if err := registry.Register("dictionary", det); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got := registry.Get("dictionary")
	if got == nil {
		t.Fatal("expected detector, got nil")
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryDetectorFuzzy (AC-005)
func TestDictionaryDetector_Fuzzy(t *testing.T) {
	ctx := context.Background()
	dict := testDict(t, []string{"password"}, dictionary.MatchModeFuzzy)
	d := NewDictionaryDetector(dict)
	results, err := d.Scan(ctx, "please enter your passw0rd")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 fuzzy match")
	}
	if results[0].Fragment != "passw0rd" {
		t.Errorf("expected 'passw0rd', got %q", results[0].Fragment)
	}
	if results[0].Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %f", results[0].Confidence)
	}
}
