package preprocessor

import (
	"strings"
	"testing"
)

// @sk-test 25-shield-preprocessors#T2.3: TestCSVProcessorFullMask (AC-001)
func TestCSVProcessorFullMask(t *testing.T) {
	p := &CSVProcessor{
		name: "csv-mask",
		rules: []Rule{
			{Columns: []string{"email", "phone"}, Mask: MaskModeFull},
		},
	}

	data := "name,email,phone\nJohn,john@test.com,555-0100\nJane,jane@test.com,555-0200"
	result := p.Process(data, "req-1")

	if !strings.Contains(result.ModifiedText, "{{csv.req-1.0}}") {
		t.Error("expected placeholder {{csv.req-1.0}} for first email")
	}
	if !strings.Contains(result.ModifiedText, "{{csv.req-1.2}}") {
		t.Error("expected placeholder for second phone")
	}
	if strings.Contains(result.ModifiedText, "john@test.com") {
		t.Error("original email should not appear in output")
	}
	if !strings.Contains(result.ModifiedText, "John") {
		t.Error("name column should remain unchanged")
	}
	if !strings.Contains(result.ModifiedText, "Jane") {
		t.Error("name column should remain unchanged")
	}
	if len(result.Replacements) == 0 {
		t.Error("expected non-empty Replacements map")
	}
}

// @sk-test 25-shield-preprocessors#T2.3: TestCSVProcessorSurnameMask (AC-002)
func TestCSVProcessorSurnameMask(t *testing.T) {
	p := &CSVProcessor{
		name: "surname-mask",
		rules: []Rule{
			{Columns: []string{"name"}, Mask: MaskModeSurname},
		},
	}

	data := "name,email\nJohn Doe,john@test.com\nJane Smith,jane@test.com"
	result := p.Process(data, "req-1")

	if !strings.Contains(result.ModifiedText, "John") {
		t.Error("expected first name 'John' to remain")
	}
	if strings.Contains(result.ModifiedText, "Doe") {
		t.Error("expected surname 'Doe' to be removed")
	}
	if !strings.Contains(result.ModifiedText, "Jane") {
		t.Error("expected first name 'Jane' to remain")
	}
	if strings.Contains(result.ModifiedText, "Smith") {
		t.Error("expected surname 'Smith' to be removed")
	}
}

// @sk-test 25-shield-preprocessors#T2.3: TestCSVProcessorQuoting (AC-006)
func TestCSVProcessorQuoting(t *testing.T) {
	p := &CSVProcessor{
		name: "csv-quote",
		rules: []Rule{
			{Columns: []string{"email"}, Mask: MaskModeFull},
		},
	}

	data := "name,email,note\n\"John \"\"Johnny\"\" Doe\",john@test.com,\"note, with comma\""
	result := p.Process(data, "req-1")

	if !strings.Contains(result.ModifiedText, "{{csv.req-1.0}}") {
		t.Error("expected placeholder for email")
	}
	if strings.Contains(result.ModifiedText, "john@test.com") {
		t.Error("original email should not appear")
	}
	if !strings.Contains(result.ModifiedText, "John \"\"Johnny\"\" Doe") {
		t.Error("quoted name field should preserve escaping")
	}
	if !strings.Contains(result.ModifiedText, "note, with comma") {
		t.Error("quoted field with comma should be preserved")
	}
}

// @sk-test 25-shield-preprocessors#T2.3: TestCSVProcessorNoMatch (edge case)
func TestCSVProcessorNoMatch(t *testing.T) {
	p := &CSVProcessor{
		name: "no-match",
		rules: []Rule{
			{Columns: []string{"nonexistent"}, Mask: MaskModeFull},
		},
	}

	data := "name,email\nJohn,john@test.com"
	result := p.Process(data, "req-1")

	if result.ModifiedText != data {
		t.Error("text should remain unchanged when no columns match")
	}
}

// @sk-test 25-shield-preprocessors#T2.3: TestCSVProcessorNoCSV (edge case)
func TestCSVProcessorNoCSV(t *testing.T) {
	p := &CSVProcessor{
		name: "no-csv",
		rules: []Rule{
			{Columns: []string{"email"}, Mask: MaskModeFull},
		},
	}

	data := "This is plain text without any CSV data."
	result := p.Process(data, "req-1")

	if result.ModifiedText != data {
		t.Error("plain text should remain unchanged")
	}
}

// @sk-test 25-shield-preprocessors#T2.3: TestCSVProcessorMultipleBlocks (edge case)
func TestCSVProcessorMultipleBlocks(t *testing.T) {
	p := &CSVProcessor{
		name: "multi-block",
		rules: []Rule{
			{Columns: []string{"secret"}, Mask: MaskModeFull},
		},
	}

	data := "header before\nsecret,value\na,123\nb,456\ntext between\nsecret,value\nc,789"
	result := p.Process(data, "req-1")

	if !strings.Contains(result.ModifiedText, "{{csv.req-1.0}}") {
		t.Error("expected placeholder in first block")
	}
	if !strings.Contains(result.ModifiedText, "{{csv.req-1.2}}") {
		t.Error("expected placeholder in second block")
	}
	if strings.Contains(result.ModifiedText, "a,123") {
		t.Error("first block value should be masked")
	}
	if strings.Contains(result.ModifiedText, "c,789") {
		t.Error("second block value should be masked")
	}
	if !strings.Contains(result.ModifiedText, "text between") {
		t.Error("interleaving text should be preserved")
	}
}
