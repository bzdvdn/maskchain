package shield

import (
	"context"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/service"
)

// @sk-task 50-shield-engine#T4.1: Implement ApplyPolicyUseCase (AC-003)
//
// ApplyPolicyUseCase represents a domain entity or configuration.
type ApplyPolicyUseCase struct {
	evaluator *service.PolicyEvaluator
}

func NewApplyPolicyUseCase(evaluator *service.PolicyEvaluator) *ApplyPolicyUseCase {
	return &ApplyPolicyUseCase{evaluator: evaluator}
}

func (uc *ApplyPolicyUseCase) Execute(ctx context.Context, result *entity.ScanResult) entity.Reaction {
	return uc.evaluator.Evaluate(result)
}
