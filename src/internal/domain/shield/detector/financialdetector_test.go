package detector

import (
	"context"
	"testing"
)

// @sk-test 21-shield-detectors#T3.2: TestFinancial finds card, IBAN, SWIFT (AC-005)
func TestFinancialDetector_FindsAllTypes(t *testing.T) {
	d, err := NewFinancialDetector()
	if err != nil {
		t.Fatal(err)
	}

	// 4532015112830366 is a valid Luhn number
	text := "Card: 4532015112830366, IBAN: GB82WEST12345698765432, SWIFT: CHASUS33"
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

	if !types["credit_card"] {
		t.Error("credit_card not found")
	}
	if !types["iban"] {
		t.Error("iban not found")
	}
	if !types["swift"] {
		t.Error("swift not found")
	}
}

// @sk-test 21-shield-detectors#T3.2: TestFinancial Luhn-invalid card excluded (AC-006)
func TestFinancialDetector_LuhnInvalid(t *testing.T) {
	d, err := NewFinancialDetector()
	if err != nil {
		t.Fatal(err)
	}

	// 4532015112830367 has last digit changed, fails Luhn
	text := "Invalid card: 4532015112830367"
	results, err := d.Scan(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		if r.DetectorType == "credit_card" {
			t.Errorf("expected no credit_card result for Luhn-invalid number, got %q", r.Fragment)
		}
	}
}

// @sk-test 21-shield-detectors#T3.2: TestFinancial empty input (AC-009)
func TestFinancialDetector_EmptyInput(t *testing.T) {
	d, err := NewFinancialDetector()
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

// @sk-test 21-shield-detectors#T3.2: TestFinancial special chars no panic (AC-010)
func TestFinancialDetector_SpecialChars(t *testing.T) {
	d, err := NewFinancialDetector()
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

// @sk-test 21-shield-detectors#T3.2: TestFinancial positions (AC-012)
func TestFinancialDetector_Positions(t *testing.T) {
	d, err := NewFinancialDetector()
	if err != nil {
		t.Fatal(err)
	}

	text := "IBAN: GB82WEST12345698765432"
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

// @sk-test 21-shield-detectors#T3.2: TestFinancial confidence (AC-011)
func TestFinancialDetector_Confidence(t *testing.T) {
	d, err := NewFinancialDetector()
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Scan(context.Background(), "4532015112830366")
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		if r.Confidence != 1.0 {
			t.Errorf("expected confidence 1.0, got %f", r.Confidence)
		}
	}
}

// @sk-test 21-shield-detectors#T3.2: TestValidLuhn standalone
func TestValidLuhn(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"4532015112830366", true},
		{"4532015112830367", false},
		{"", false},
		{"0", true},
	}
	for _, tc := range tests {
		got := validLuhn(tc.input)
		if got != tc.valid {
			t.Errorf("validLuhn(%q) = %v, want %v", tc.input, got, tc.valid)
		}
	}
}

// @sk-test 21-shield-detectors#T3.2: TestStripNonDigits
func TestStripNonDigits(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"4532-0151-1283-0366", "4532015112830366"},
		{"", ""},
		{"abc", ""},
		{"12 34", "1234"},
	}
	for _, tc := range tests {
		got := stripNonDigits(tc.input)
		if got != tc.want {
			t.Errorf("stripNonDigits(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
