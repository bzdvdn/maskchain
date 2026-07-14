package shield

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 50-shield-engine#T2.2: TestFullPipeline (AC-001)
// @sk-test 13-shield-middleware-wiring#T1.3: Empty rules → clean (AC-007)
func TestScanUseCase_EmptyRules(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)
	factory := NewScanPipelineFactory(registry)
	uc := NewScanUseCase(factory)

	resp, err := uc.Scan(ctx, ScanRequest{Text: "hello world", Rules: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status() != value.ScanStatusClean {
		t.Errorf("expected clean, got %v", resp.Status())
	}
	if len(resp.Incidents()) != 0 {
		t.Errorf("expected 0 incidents, got %d", len(resp.Incidents()))
	}
}

// @sk-test 50-shield-engine#T2.2: TestFullPipeline (AC-001)
// @sk-test 13-shield-middleware-wiring#T1.3: PII detection with Rules (AC-001)
func TestScanUseCase_PIIDetection(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)
	factory := NewScanPipelineFactory(registry)
	uc := NewScanUseCase(factory)

	text := "email,phone,notes\ntest@example.com,+1-555-123-4567,handle secret123 ended\n"
	resp, err := uc.Scan(ctx, ScanRequest{
		Text: text,
		Rules: []entity.PIARule{
			{Label: "pii-catch-all", Type: "pii", Pattern: ".*", Action: "block"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status() != value.ScanStatusSuspicious && resp.Status() != value.ScanStatusBlocked {
		t.Errorf("expected suspicious or blocked, got %v", resp.Status())
	}
	if len(resp.Incidents()) < 1 {
		t.Errorf("expected at least 1 incident (PII), got %d", len(resp.Incidents()))
	}
}

// @sk-test 50-shield-engine#T2.2: TestProfileNotFound (AC-004) — now tests unknown detector type
// @sk-test 13-shield-middleware-wiring#T1.3: Unknown detector type → error
func TestScanUseCase_UnknownDetectorType(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)
	factory := NewScanPipelineFactory(registry)
	uc := NewScanUseCase(factory)

	_, err := uc.Scan(ctx, ScanRequest{
		Text: "hello",
		Rules: []entity.PIARule{
			{Label: "unknown", Type: "nonexistent", Pattern: ".*", Action: "block"},
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown detector type, got nil")
	}
}

// @sk-test 50-shield-engine#T2.2: TestEmptyPipeline (AC-005)
// @sk-test 13-shield-middleware-wiring#T1.3: Empty rules slice → clean (AC-007)
func TestScanUseCase_EmptyRulesSlice(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)
	factory := NewScanPipelineFactory(registry)
	uc := NewScanUseCase(factory)

	resp, err := uc.Scan(ctx, ScanRequest{Text: "some text", Rules: []entity.PIARule{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status() != value.ScanStatusClean {
		t.Errorf("expected clean status for empty rules, got %v", resp.Status())
	}
	if len(resp.Incidents()) != 0 {
		t.Errorf("expected 0 incidents for empty rules, got %d", len(resp.Incidents()))
	}
}

// --- test helpers ---

func setupRegistry(t *testing.T) (*detector.DetectorRegistry, entity.DetectorType) {
	t.Helper()
	reg := detector.NewDetectorRegistry()
	pii, err := detector.NewPIIDetector()
	if err != nil {
		t.Fatalf("new PII detector: %v", err)
	}
	piiType := entity.DetectorType("pii")
	if err := reg.Register(piiType, pii); err != nil {
		t.Fatalf("register PII: %v", err)
	}
	return reg, piiType
}
