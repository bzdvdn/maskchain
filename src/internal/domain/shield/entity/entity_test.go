package entity

// @sk-task cleanup-profile-repository#T3.6: Remove Profile entity tests (AC-012)
import (
	"errors"
	"testing"

	domErr "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 20-shield-domain#T5.2: TestNewDetector valid (AC-008)
func TestNewDetector_Valid(t *testing.T) {
	patID, _ := value.NewPatternID("pat-1")
	pat, err := NewPattern(patID, "secret.*", "secret pattern")
	if err != nil {
		t.Fatal(err)
	}

	det, err := NewDetector("det-1", DetectorTypeRegex, []Pattern{*pat}, value.SeverityHigh)
	if err != nil {
		t.Fatal(err)
	}
	if det.ID() != "det-1" {
		t.Errorf("expected ID det-1, got %q", det.ID())
	}
	if det.Type() != DetectorTypeRegex {
		t.Errorf("expected DetectorTypeRegex, got %v", det.Type())
	}
	if len(det.Patterns()) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(det.Patterns()))
	}
	if det.Severity() != value.SeverityHigh {
		t.Errorf("expected SeverityHigh, got %v", det.Severity())
	}
}

// @sk-test 20-shield-domain#T5.2: TestNewDetector without patterns returns error (AC-008)
func TestNewDetector_NoPatterns(t *testing.T) {
	_, err := NewDetector("det-2", DetectorTypeKeyword, nil, value.SeverityLow)
	if err == nil {
		t.Fatal("expected error for detector without patterns")
	}
	if !errors.Is(err, domErr.ErrInvalidPattern) {
		t.Errorf("expected ErrInvalidPattern, got %v", err)
	}
}

// @sk-test 20-shield-domain#T5.2: TestNewPattern valid (AC-008)
func TestNewPattern_Valid(t *testing.T) {
	patID, _ := value.NewPatternID("pat-2")
	pat, err := NewPattern(patID, `\d{3}-\d{2}-\d{4}`, "SSN pattern")
	if err != nil {
		t.Fatal(err)
	}
	if pat.Expression() != `\d{3}-\d{2}-\d{4}` {
		t.Errorf("unexpected expression: %q", pat.Expression())
	}
}

// @sk-test 20-shield-domain#T5.2: TestNewPattern empty expression
func TestNewPattern_EmptyExpression(t *testing.T) {
	patID, _ := value.NewPatternID("pat-3")
	_, err := NewPattern(patID, "", "")
	if err == nil {
		t.Fatal("expected error for empty expression")
	}
	if !errors.Is(err, domErr.ErrInvalidPattern) {
		t.Errorf("expected ErrInvalidPattern, got %v", err)
	}
}

// @sk-test remove-audit-incidents#T4.1: TestNewScanResult without incidents (AC-014)
func TestNewScanResult(t *testing.T) {
	result := NewScanResult(value.ScanStatusSuspicious)
	if result.Status() != value.ScanStatusSuspicious {
		t.Errorf("expected Suspicious status")
	}
	if result.ScannedAt().IsZero() {
		t.Error("expected non-zero ScannedAt")
	}
}

// @sk-test 20-shield-domain#T5.2: TestReaction constants
func TestReaction_Constants(t *testing.T) {
	if ReactionAllow != "allow" {
		t.Errorf("expected allow, got %q", ReactionAllow)
	}
}
