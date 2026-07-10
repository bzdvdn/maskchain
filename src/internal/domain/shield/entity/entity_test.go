package entity

import (
	"errors"
	"testing"
	"time"

	domErr "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 20-shield-domain#T5.2: TestNewProfile creates with valid fields (AC-001)
func TestNewProfile_Valid(t *testing.T) {
	id, _ := value.NewProfileID("p1")
	slug, _ := value.NewProfileSlug("test-profile")
	tenant, _ := value.NewTenantID("t1")

	p := NewProfile(id, slug, tenant, "Test Profile")
	if p.ID() != id {
		t.Errorf("expected ID %v, got %v", id, p.ID())
	}
	if p.Slug() != slug {
		t.Errorf("expected Slug %v, got %v", slug, p.Slug())
	}
	if p.TenantID() != tenant {
		t.Errorf("expected TenantID %v, got %v", tenant, p.TenantID())
	}
	if p.Name() != "Test Profile" {
		t.Errorf("expected Name 'Test Profile', got %q", p.Name())
	}
	if !p.Enabled() {
		t.Error("expected Enabled=true by default")
	}
	if p.CreatedAt().IsZero() {
		t.Error("expected CreatedAt non-zero")
	}
	if p.UpdatedAt().IsZero() {
		t.Error("expected UpdatedAt non-zero")
	}
}

// @sk-test 20-shield-domain#T5.2: TestNewProfile with options (AC-001)
func TestNewProfile_WithOptions(t *testing.T) {
	id, _ := value.NewProfileID("p2")
	slug, _ := value.NewProfileSlug("with-opts")
	tenant, _ := value.NewTenantID("t1")

	desc := "my description"
	p := NewProfile(id, slug, tenant, "With Options",
		WithDescription(desc),
		WithEnabled(false),
	)
	if p.Description() == nil || *p.Description() != desc {
		t.Errorf("expected description %q, got %v", desc, p.Description())
	}
	if p.Enabled() {
		t.Error("expected Enabled=false")
	}
}

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

// @sk-test 20-shield-domain#T5.2: TestNewIncident
func TestNewIncident(t *testing.T) {
	patID, _ := value.NewPatternID("pat-1")
	inc := NewIncident("det-1", patID, value.SeverityHigh, "sensitive data", 42)
	if inc.DetectorID() != "det-1" {
		t.Errorf("expected detectorID det-1, got %q", inc.DetectorID())
	}
	if inc.PatternID() != patID {
		t.Errorf("unexpected patternID")
	}
	if inc.Severity() != value.SeverityHigh {
		t.Errorf("expected SeverityHigh")
	}
	if inc.Fragment() != "sensitive data" {
		t.Errorf("unexpected fragment")
	}
	if inc.Position() != 42 {
		t.Errorf("expected position 42")
	}
}

// @sk-test 20-shield-domain#T5.2: TestNewScanResult
func TestNewScanResult(t *testing.T) {
	patID, _ := value.NewPatternID("pat-1")
	inc := NewIncident("det-1", patID, value.SeverityLow, "test", 0)
	result := NewScanResult(value.ScanStatusSuspicious, []Incident{*inc})
	if result.Status() != value.ScanStatusSuspicious {
		t.Errorf("expected Suspicious status")
	}
	if len(result.Incidents()) != 1 {
		t.Errorf("expected 1 incident")
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

// @sk-test 20-shield-domain#T5.2: TestProfile timestamps are set on creation (AC-001)
func TestProfile_Timestamps(t *testing.T) {
	id, _ := value.NewProfileID("p3")
	slug, _ := value.NewProfileSlug("timestamps")
	tenant, _ := value.NewTenantID("t1")

	before := time.Now()
	p := NewProfile(id, slug, tenant, "Timestamps")
	after := time.Now()

	if p.CreatedAt().Before(before) || p.CreatedAt().After(after) {
		t.Error("CreatedAt should be between before and after")
	}
}
