package shield

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 50-shield-engine#T4.2: TestApplyPolicyUseCase (AC-003)
func TestApplyPolicyUseCase_ReturnsReactionByHighestSeverity(t *testing.T) {
	ctx := context.Background()
	evaluator := service.NewPolicyEvaluator()
	uc := NewApplyPolicyUseCase(evaluator)

	incLow := entity.NewIncident("d1", value.PatternID{}, value.SeverityLow, "low", 0)
	incMedium := entity.NewIncident("d2", value.PatternID{}, value.SeverityMedium, "medium", 0)
	incHigh := entity.NewIncident("d3", value.PatternID{}, value.SeverityHigh, "high", 0)

	t.Run("no incidents returns allow", func(t *testing.T) {
		result := entity.NewScanResult(value.ScanStatusClean, nil)
		reaction := uc.Execute(ctx, result)
		if reaction != entity.ReactionAllow {
			t.Errorf("expected allow, got %v", reaction)
		}
	})

	t.Run("low severity returns log", func(t *testing.T) {
		inc := entity.NewIncident("d1", value.PatternID{}, value.SeverityLow, "low", 0)
		result := entity.NewScanResult(value.ScanStatusSuspicious, []entity.Incident{*inc})
		reaction := uc.Execute(ctx, result)
		if reaction != entity.ReactionLog {
			t.Errorf("expected log for low severity, got %v", reaction)
		}
	})

	t.Run("medium severity returns log", func(t *testing.T) {
		result := entity.NewScanResult(value.ScanStatusSuspicious, []entity.Incident{*incMedium})
		reaction := uc.Execute(ctx, result)
		if reaction != entity.ReactionLog {
			t.Errorf("expected log for medium severity, got %v", reaction)
		}
	})

	t.Run("high severity returns review", func(t *testing.T) {
		result := entity.NewScanResult(value.ScanStatusSuspicious, []entity.Incident{*incHigh})
		reaction := uc.Execute(ctx, result)
		if reaction != entity.ReactionReview {
			t.Errorf("expected review for high severity, got %v", reaction)
		}
	})

	t.Run("mixed severities picks highest", func(t *testing.T) {
		result := entity.NewScanResult(value.ScanStatusSuspicious, []entity.Incident{*incLow, *incMedium, *incHigh})
		reaction := uc.Execute(ctx, result)
		if reaction != entity.ReactionReview {
			t.Errorf("expected review (highest=high), got %v", reaction)
		}
	})
}
