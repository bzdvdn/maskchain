package value

import (
	"testing"
)

// @sk-test 20-shield-domain#T5.1: TestProfileID equality (AC-006)
func TestProfileID_Equality(t *testing.T) {
	a, _ := NewProfileID("abc-123")
	b, _ := NewProfileID("abc-123")
	if a != b {
		t.Error("expected equal ProfileID for same value")
	}
}

// @sk-test 20-shield-domain#T5.1: TestProfileID empty error
func TestProfileID_Empty(t *testing.T) {
	_, err := NewProfileID("")
	if err == nil {
		t.Error("expected error for empty ProfileID")
	}
}

// @sk-test 20-shield-domain#T5.1: TestProfileSlug validation (AC-002)
func TestProfileSlug_Valid(t *testing.T) {
	cases := []struct {
		input string
		valid bool
	}{
		{"my-profile", true},
		{"abc123", true},
		{"test-42", true},
		{"", false},
		{"ab", false},
		{"hello world", false},
		{"привет", false},
		{"profile!@#", false},
	}
	for _, c := range cases {
		_, err := NewProfileSlug(c.input)
		if c.valid && err != nil {
			t.Errorf("expected valid slug %q, got error: %v", c.input, err)
		}
		if !c.valid && err == nil {
			t.Errorf("expected error for invalid slug %q", c.input)
		}
	}
}

// @sk-test 20-shield-domain#T5.1: TestProfileSlug equality (AC-006)
func TestProfileSlug_Equality(t *testing.T) {
	a, _ := NewProfileSlug("my-slug")
	b, _ := NewProfileSlug("my-slug")
	if a != b {
		t.Error("expected equal ProfileSlug for same value")
	}
}

// @sk-test 20-shield-domain#T5.1: TestTenantID equality (AC-006)
func TestTenantID_Equality(t *testing.T) {
	a, _ := NewTenantID("tenant-1")
	b, _ := NewTenantID("tenant-1")
	if a != b {
		t.Error("expected equal TenantID for same value")
	}
}

// @sk-test 20-shield-domain#T5.1: TestSeverity ordering
func TestSeverity_Ordering(t *testing.T) {
	if SeverityLow >= SeverityMedium {
		t.Error("expected SeverityLow < SeverityMedium")
	}
	if SeverityMedium >= SeverityHigh {
		t.Error("expected SeverityMedium < SeverityHigh")
	}
	if SeverityHigh >= SeverityCritical {
		t.Error("expected SeverityHigh < SeverityCritical")
	}
}

// @sk-test 20-shield-domain#T5.1: TestSeverity string representation
func TestSeverity_String(t *testing.T) {
	cases := []struct {
		s    Severity
		want string
	}{
		{SeverityLow, "low"},
		{SeverityMedium, "medium"},
		{SeverityHigh, "high"},
		{SeverityCritical, "critical"},
		{Severity(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("Severity(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

// @sk-test 20-shield-domain#T5.1: TestScanStatus constants
func TestScanStatus_Values(t *testing.T) {
	if ScanStatusClean != "clean" {
		t.Errorf("expected ScanStatusClean=clean, got %q", ScanStatusClean)
	}
}
