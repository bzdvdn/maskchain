package detector

import (
	"context"
	"testing"
)

// @sk-test 21-shield-detectors#T3.1: TestSecrets finds API key, JWT, PEM (AC-004)
func TestSecretsDetector_FindsAllTypes(t *testing.T) {
	d, err := NewSecretsDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "key sk-abc123def456ghijkl and JWT eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNrvP5FQw0QJ0Q0J0Q and PEM -----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQ==\n-----END PRIVATE KEY-----"
	results, err := d.Scan(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	types := make(map[string]bool)
	for _, r := range results {
		types[r.DetectorType] = true
	}

	if !types["api_key"] {
		t.Error("api_key not found")
	}
	if !types["jwt"] {
		t.Error("jwt not found")
	}
	if !types["private_key"] {
		t.Error("private_key not found")
	}
}

// @sk-test 21-shield-detectors#T3.1: TestSecrets empty input (AC-009)
func TestSecretsDetector_EmptyInput(t *testing.T) {
	d, err := NewSecretsDetector()
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Scan(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}

	if results == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// @sk-test 21-shield-detectors#T3.1: TestSecrets special chars no panic (AC-010)
func TestSecretsDetector_SpecialChars(t *testing.T) {
	d, err := NewSecretsDetector()
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Scan(context.Background(), "\x00\n\t\r!@#$%^&*()_+={}[]|\\:;\"'<>,.?/~`")
	if err != nil {
		t.Fatal(err)
	}

	if results == nil {
		t.Fatal("expected non-nil slice")
	}
}

// @sk-test 21-shield-detectors#T3.1: TestSecrets confidence (AC-011)
func TestSecretsDetector_Confidence(t *testing.T) {
	d, err := NewSecretsDetector()
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Scan(context.Background(), "sk-testkey1234567890abcdef")
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		if r.Confidence != 1.0 {
			t.Errorf("expected confidence 1.0, got %f", r.Confidence)
		}
	}
}

// @sk-test 21-shield-detectors#T3.1: TestSecrets positions (AC-012)
func TestSecretsDetector_Positions(t *testing.T) {
	d, err := NewSecretsDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "token sk-abcdef1234567890 end"
	results, err := d.Scan(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if text[r.StartPos:r.EndPos] != r.Fragment {
		t.Errorf("position mismatch: text[%d:%d]=%q, fragment=%q", r.StartPos, r.EndPos, text[r.StartPos:r.EndPos], r.Fragment)
	}
}
