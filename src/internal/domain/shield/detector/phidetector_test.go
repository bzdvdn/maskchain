package detector

import (
	"context"
	"testing"
)

// @sk-test 21-shield-detectors#T3.3: TestPHI finds ICD-10 codes (AC-007)
func TestPHIDetector_FindsICD10(t *testing.T) {
	d, err := NewPHIDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "Codes: A00.0, B99.9, J45.0"
	results, err := d.Scan(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	fragments := make(map[string]bool)
	for _, r := range results {
		fragments[r.Fragment] = true
	}

	if !fragments["A00.0"] {
		t.Error("A00.0 not found")
	}
	if !fragments["B99.9"] {
		t.Error("B99.9 not found")
	}
	if !fragments["J45.0"] {
		t.Error("J45.0 not found")
	}
}

// @sk-test 21-shield-detectors#T3.3: TestPHI empty input (AC-009)
func TestPHIDetector_EmptyInput(t *testing.T) {
	d, err := NewPHIDetector()
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

// @sk-test 21-shield-detectors#T3.3: TestPHI special chars no panic (AC-010)
func TestPHIDetector_SpecialChars(t *testing.T) {
	d, err := NewPHIDetector()
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

// @sk-test 21-shield-detectors#T3.3: TestPHI confidence (AC-011)
func TestPHIDetector_Confidence(t *testing.T) {
	d, err := NewPHIDetector()
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Scan(context.Background(), "A00.0")
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		if r.Confidence != 1.0 {
			t.Errorf("expected confidence 1.0, got %f", r.Confidence)
		}
	}
}

// @sk-test 21-shield-detectors#T3.3: TestPHI positions (AC-012)
func TestPHIDetector_Positions(t *testing.T) {
	d, err := NewPHIDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "Diagnosis: J45.0"
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
