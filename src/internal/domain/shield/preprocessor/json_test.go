package preprocessor

import (
	"strings"
	"testing"
)

// @sk-test 25-shield-preprocessors#T2.4: TestJSONProcessorNestedField (AC-003)
func TestJSONProcessorNestedField(t *testing.T) {
	p := &JSONProcessor{
		name: "json-mask",
		rules: []Rule{
			{Path: "user.email", Mask: MaskModeFull},
		},
	}

	data := `{"user": {"email": "test@test.com", "name": "John"}}`
	result := p.Process(data, "req-1")

	if !strings.Contains(result.ModifiedText, "[MASK_") {
		t.Error("expected placeholder in output")
	}
	if strings.Contains(result.ModifiedText, "test@test.com") {
		t.Error("original email should not appear")
	}
	if !strings.Contains(result.ModifiedText, "John") {
		t.Error("name field should remain unchanged")
	}
}

// @sk-test 25-shield-preprocessors#T2.4: TestJSONProcessorMarkdownFence (AC-004)
func TestJSONProcessorMarkdownFence(t *testing.T) {
	p := &JSONProcessor{
		name: "json-fence",
		rules: []Rule{
			{Path: "secret", Mask: MaskModeFull},
		},
	}

	data := "some text\n```json\n{\"secret\": \"sensitive\"}\n```\nmore text"
	result := p.Process(data, "req-1")

	if !strings.Contains(result.ModifiedText, "[MASK_") {
		t.Error("expected placeholder in output")
	}
	if strings.Contains(result.ModifiedText, "sensitive") {
		t.Error("original secret should not appear")
	}
	if !strings.Contains(result.ModifiedText, "```json") {
		t.Error("markdown fence should be preserved")
	}
	if !strings.Contains(result.ModifiedText, "some text") || !strings.Contains(result.ModifiedText, "more text") {
		t.Error("text outside fence should be preserved")
	}
}

// @sk-test 25-shield-preprocessors#T2.4: TestJSONProcessorWildcard (AC-005)
func TestJSONProcessorWildcard(t *testing.T) {
	p := &JSONProcessor{
		name: "json-wildcard",
		rules: []Rule{
			{Path: "items[*].secret", Mask: MaskModeFull},
		},
	}

	data := `{"items": [{"secret": "a"}, {"secret": "b"}]}`
	result := p.Process(data, "req-1")

	if !strings.Contains(result.ModifiedText, "[MASK_json") {
		t.Error("expected placeholder in output")
	}
	if strings.Contains(result.ModifiedText, "\"secret\": \"a\"") {
		t.Error("first secret should be masked")
	}
	if strings.Contains(result.ModifiedText, "\"secret\": \"b\"") {
		t.Error("second secret should be masked")
	}
}

// @sk-test 25-shield-preprocessors#T2.4: TestJSONProcessorTopLevelField (edge case)
func TestJSONProcessorTopLevelField(t *testing.T) {
	p := &JSONProcessor{
		name: "json-top",
		rules: []Rule{
			{Path: "email", Mask: MaskModeFull},
		},
	}

	data := `{"email": "test@test.com", "name": "John"}`
	result := p.Process(data, "req-1")

	if !strings.Contains(result.ModifiedText, "[MASK_") {
		t.Error("expected placeholder for email")
	}
	if strings.Contains(result.ModifiedText, "test@test.com") {
		t.Error("original email should not appear")
	}
}

// @sk-test 25-shield-preprocessors#T2.4: TestJSONProcessorNoMatch (edge case)
func TestJSONProcessorNoMatch(t *testing.T) {
	p := &JSONProcessor{
		name: "json-nomatch",
		rules: []Rule{
			{Path: "nonexistent.field", Mask: MaskModeFull},
		},
	}

	data := `{"user": {"email": "test@test.com"}}`
	result := p.Process(data, "req-1")

	if result.ModifiedText != data {
		t.Error("text should remain unchanged when path doesn't match")
	}
}

// @sk-test 25-shield-preprocessors#T2.4: TestJSONProcessorPlainTextNoMatch
func TestJSONProcessorPlainTextNoMatch(t *testing.T) {
	p := &JSONProcessor{
		name: "json-plain",
		rules: []Rule{
			{Path: "email", Mask: MaskModeFull},
		},
	}

	data := "This is plain text without JSON"
	result := p.Process(data, "req-1")

	if result.ModifiedText != data {
		t.Error("plain text should remain unchanged")
	}
}
