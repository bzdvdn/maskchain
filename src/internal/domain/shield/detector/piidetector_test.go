package detector

import (
	"context"
	"testing"
)

// @sk-test 21-shield-detectors#T2.2: TestPII finds email, phone, SSN, passport (AC-003)
func TestPIIDetector_FindsAllTypes(t *testing.T) {
	d, err := NewPIIDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "Email: test@example.com, Phone: +7 (999) 123-45-67, SSN: 123-45-6789, Passport: 1234 567890"
	results, err := d.Scan(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	types := make(map[string]bool)
	for _, r := range results {
		types[r.DetectorType] = true
		if r.Confidence != 1.0 {
			t.Errorf("expected confidence 1.0 for %s, got %f", r.DetectorType, r.Confidence)
		}
		if text[r.StartPos:r.EndPos] != r.Fragment {
			t.Errorf("position mismatch for %s: text[%d:%d] != %q", r.DetectorType, r.StartPos, r.EndPos, r.Fragment)
		}
	}

	if !types["email"] {
		t.Error("email not found")
	}
	if !types["phone"] {
		t.Error("phone not found")
	}
	if !types["ssn"] {
		t.Error("ssn not found")
	}
	if !types["passport"] {
		t.Error("passport not found")
	}
}

// @sk-test 21-shield-detectors#T2.2: TestPII empty input returns empty slice (AC-009)
func TestPIIDetector_EmptyInput(t *testing.T) {
	d, err := NewPIIDetector()
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Scan(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}

	if results == nil {
		t.Fatal("expected non-nil slice for empty input")
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// @sk-test 21-shield-detectors#T2.2: TestPII special characters no panic (AC-010)
func TestPIIDetector_SpecialChars(t *testing.T) {
	d, err := NewPIIDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "\x00\n\t\r!@#$%^&*()_+={}[]|\\:;\"'<>,.?/~`"
	results, err := d.Scan(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if results == nil {
		t.Fatal("expected non-nil slice")
	}
}

// @sk-test 21-shield-detectors#T2.2: TestPII positions are correct (AC-012)
func TestPIIDetector_Positions(t *testing.T) {
	d, err := NewPIIDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "My email is test@example.com"
	results, err := d.Scan(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if text[r.StartPos:r.EndPos] != "test@example.com" {
		t.Errorf("position mismatch: got %q", text[r.StartPos:r.EndPos])
	}
	if r.Fragment != "test@example.com" {
		t.Errorf("fragment mismatch: got %q", r.Fragment)
	}
}

// @sk-test 21-shield-detectors#T2.2: TestPII partial matches excluded (AC-003 edge)
func TestPIIDetector_PartialMatches(t *testing.T) {
	d, err := NewPIIDetector()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		text string
		want int
	}{
		{"incomplete email", "contact me at test@example", 0},
		{"incomplete SSN", "123-45 not full SSN", 0},
		{"no PII", "just regular text here", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := d.Scan(context.Background(), tc.text)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != tc.want {
				t.Errorf("expected %d results, got %d", tc.want, len(results))
			}
		})
	}
}

// @sk-test 21-shield-detectors#T2.2: TestPII confidence is 1.0 (AC-011)
func TestPIIDetector_Confidence(t *testing.T) {
	d, err := NewPIIDetector()
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Scan(context.Background(), "email: user@domain.com")
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		if r.Confidence != 1.0 {
			t.Errorf("expected confidence 1.0, got %f", r.Confidence)
		}
	}
}

// @sk-test 21-shield-detectors#T2.2: TestPII overlapping patterns handled (AC-010 edge)
func TestPIIDetector_OverlappingPatterns(t *testing.T) {
	d, err := NewPIIDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "contact +7-999-123-45-67 or 123-45-6789"
	results, err := d.Scan(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}
}
