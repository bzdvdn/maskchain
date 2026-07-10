package preprocessor

import (
	"testing"
)

// @sk-test 25-shield-preprocessors#T2.5: TestNewPreprocessorCSV (AC-007)
func TestNewPreprocessorCSV(t *testing.T) {
	def := PreprocessorDef{
		Name: "csv-mask",
		Type: "csv",
		Rules: []Rule{
			{Columns: []string{"email"}, Mask: MaskModeFull},
		},
	}

	p, err := NewPreprocessor(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "csv-mask" {
		t.Errorf("expected name 'csv-mask', got %q", p.Name())
	}
	if _, ok := p.(*CSVProcessor); !ok {
		t.Error("expected *CSVProcessor type")
	}
}

// @sk-test 25-shield-preprocessors#T2.5: TestNewPreprocessorJSON (AC-007)
func TestNewPreprocessorJSON(t *testing.T) {
	def := PreprocessorDef{
		Name: "json-mask",
		Type: "json",
		Rules: []Rule{
			{Path: "user.email", Mask: MaskModeFull},
		},
	}

	p, err := NewPreprocessor(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "json-mask" {
		t.Errorf("expected name 'json-mask', got %q", p.Name())
	}
	if _, ok := p.(*JSONProcessor); !ok {
		t.Error("expected *JSONProcessor type")
	}
}

// @sk-test 25-shield-preprocessors#T2.5: TestNewPreprocessorUnknownType (AC-007)
func TestNewPreprocessorUnknownType(t *testing.T) {
	def := PreprocessorDef{
		Name: "unknown",
		Type: "xml",
		Rules: []Rule{
			{Path: "field", Mask: MaskModeFull},
		},
	}

	_, err := NewPreprocessor(def)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

// @sk-test 25-shield-preprocessors#T2.5: TestNewPreprocessorEmptyName (edge case)
func TestNewPreprocessorEmptyName(t *testing.T) {
	def := PreprocessorDef{
		Name: "",
		Type: "csv",
		Rules: []Rule{
			{Columns: []string{"email"}, Mask: MaskModeFull},
		},
	}

	_, err := NewPreprocessor(def)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

// @sk-test 25-shield-preprocessors#T2.5: TestNewPreprocessorEmptyRules (edge case)
func TestNewPreprocessorEmptyRules(t *testing.T) {
	def := PreprocessorDef{
		Name:  "empty",
		Type:  "csv",
		Rules: []Rule{},
	}

	_, err := NewPreprocessor(def)
	if err == nil {
		t.Fatal("expected error for empty rules")
	}
}

// @sk-test 25-shield-preprocessors#T2.5: TestNewPreprocessorInvalidMaskMode (edge case)
func TestNewPreprocessorInvalidMaskMode(t *testing.T) {
	def := PreprocessorDef{
		Name: "bad-mode",
		Type: "csv",
		Rules: []Rule{
			{Columns: []string{"email"}, Mask: "invalid"},
		},
	}

	_, err := NewPreprocessor(def)
	if err == nil {
		t.Fatal("expected error for invalid mask mode")
	}
}
