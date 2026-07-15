package value

// @sk-task cleanup-profile-repository#T3.6: Remove ProfileSlug/ProfileID value tests (AC-012)
import (
	"testing"
)

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
