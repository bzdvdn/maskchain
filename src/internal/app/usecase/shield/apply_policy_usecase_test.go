package shield

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 50-shield-engine#T4.2: TestApplyPolicyUseCase (AC-003)
// @sk-test remove-audit-incidents#T4.1: Use ScanStatus instead of incidents (AC-014)
func TestApplyPolicyUseCase_ReturnsReactionByStatus(t *testing.T) {
	ctx := context.Background()
	evaluator := service.NewPolicyEvaluator()
	uc := NewApplyPolicyUseCase(evaluator)

	t.Run("clean returns allow", func(t *testing.T) {
		result := entity.NewScanResult(value.ScanStatusClean)
		reaction := uc.Execute(ctx, result)
		if reaction != entity.ReactionAllow {
			t.Errorf("expected allow, got %v", reaction)
		}
	})

	t.Run("blocked returns block", func(t *testing.T) {
		result := entity.NewScanResult(value.ScanStatusBlocked)
		reaction := uc.Execute(ctx, result)
		if reaction != entity.ReactionBlock {
			t.Errorf("expected block, got %v", reaction)
		}
	})

	t.Run("suspicious returns review", func(t *testing.T) {
		result := entity.NewScanResult(value.ScanStatusSuspicious)
		reaction := uc.Execute(ctx, result)
		if reaction != entity.ReactionReview {
			t.Errorf("expected review, got %v", reaction)
		}
	})
}
