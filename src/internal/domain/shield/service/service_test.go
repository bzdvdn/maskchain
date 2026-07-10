package service

import (
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

func makePattern(t *testing.T, id, expr string) entity.Pattern {
	t.Helper()
	pid, _ := value.NewPatternID(id)
	pat, err := entity.NewPattern(pid, expr, "")
	if err != nil {
		t.Fatal(err)
	}
	return *pat
}

func makeDetector(t *testing.T, id string, typ entity.DetectorType, patterns []entity.Pattern, sev value.Severity, enabled ...bool) entity.Detector {
	t.Helper()
	var opts []entity.DetectorOption
	if len(enabled) > 0 {
		opts = append(opts, entity.WithDetectorEnabled(enabled[0]))
	}
	det, err := entity.NewDetector(id, typ, patterns, sev, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return *det
}

// @sk-test 20-shield-domain#T5.3: TestScanPipeline with enabled detectors (AC-003)
func TestScanPipeline_Execute(t *testing.T) {
	pipeline := NewScanPipeline()
	pat := makePattern(t, "p1", "secret")
	det := makeDetector(t, "d1", entity.DetectorTypeRegex, []entity.Pattern{pat}, value.SeverityHigh)

	result := pipeline.Execute([]entity.Detector{det}, "some content")
	if result.Status() != value.ScanStatusClean {
		t.Errorf("expected clean with simple content, got %v", result.Status())
	}
}

// @sk-test 20-shield-domain#T5.3: TestScanPipeline empty detectors (AC-003)
func TestScanPipeline_EmptyDetectors(t *testing.T) {
	pipeline := NewScanPipeline()
	result := pipeline.Execute(nil, "content")
	if result.Status() != value.ScanStatusClean {
		t.Errorf("expected clean for no detectors, got %v", result.Status())
	}
}

// @sk-test 20-shield-domain#T5.3: TestScanPipeline disabled detector skipped (AC-003)
func TestScanPipeline_DisabledDetector(t *testing.T) {
	pipeline := NewScanPipeline()
	pat := makePattern(t, "p2", "secret")
	det := makeDetector(t, "d2", entity.DetectorTypeRegex, []entity.Pattern{pat}, value.SeverityHigh, false)

	result := pipeline.Execute([]entity.Detector{det}, "secret content")
	// disabled detector should not match
	if result.Status() != value.ScanStatusClean {
		t.Errorf("expected clean for disabled detector, got %v", result.Status())
	}
}

// @sk-test 20-shield-domain#T5.3: TestPolicyEvaluator no incidents (AC-004)
func TestPolicyEvaluator_NoIncidents(t *testing.T) {
	eval := NewPolicyEvaluator()
	result := entity.NewScanResult(value.ScanStatusClean, nil)
	reaction := eval.Evaluate(result)
	if reaction != entity.ReactionAllow {
		t.Errorf("expected allow for clean result, got %v", reaction)
	}
}

// @sk-test 20-shield-domain#T5.3: TestPolicyEvaluator nil result (AC-004)
func TestPolicyEvaluator_NilResult(t *testing.T) {
	eval := NewPolicyEvaluator()
	reaction := eval.Evaluate(nil)
	if reaction != entity.ReactionBlock {
		t.Errorf("expected block for nil result, got %v", reaction)
	}
}

// @sk-test 20-shield-domain#T5.3: TestPolicyEvaluator severity mapping (AC-004)
func TestPolicyEvaluator_SeverityMapping(t *testing.T) {
	eval := NewPolicyEvaluator()
	pid, _ := value.NewPatternID("p-eval")

	tests := []struct {
		severity value.Severity
		want     entity.Reaction
	}{
		{value.SeverityCritical, entity.ReactionBlock},
		{value.SeverityHigh, entity.ReactionReview},
		{value.SeverityMedium, entity.ReactionLog},
		{value.SeverityLow, entity.ReactionLog},
	}

	for _, tt := range tests {
		inc := entity.NewIncident("det-eval", pid, tt.severity, "test", 0)
		result := entity.NewScanResult(value.ScanStatusSuspicious, []entity.Incident{*inc})
		got := eval.Evaluate(result)
		if got != tt.want {
			t.Errorf("Evaluate with severity %v: expected %v, got %v", tt.severity, tt.want, got)
		}
	}
}

// @sk-test 20-shield-domain#T5.3: TestPolicyEvaluator highest severity wins (AC-004)
func TestPolicyEvaluator_HighestSeverity(t *testing.T) {
	eval := NewPolicyEvaluator()
	pid, _ := value.NewPatternID("p-highest")

	low := entity.NewIncident("det-low", pid, value.SeverityLow, "low", 0)
	high := entity.NewIncident("det-high", pid, value.SeverityHigh, "high", 0)
	result := entity.NewScanResult(value.ScanStatusSuspicious, []entity.Incident{*low, *high})

	reaction := eval.Evaluate(result)
	if reaction != entity.ReactionReview {
		t.Errorf("expected review for highest=high, got %v", reaction)
	}
}
