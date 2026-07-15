package service

import (
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T4.2: Implement PolicyEvaluator service (AC-004)
type PolicyEvaluator struct{}

func NewPolicyEvaluator() *PolicyEvaluator {
	return &PolicyEvaluator{}
}

// @sk-task remove-audit-incidents#T1.4: Use ScanStatus instead of incidents for evaluation (AC-006)
func (e *PolicyEvaluator) Evaluate(result *entity.ScanResult) entity.Reaction {
	if result == nil {
		return entity.ReactionBlock
	}

	switch result.Status() {
	case value.ScanStatusClean:
		return entity.ReactionAllow
	case value.ScanStatusBlocked:
		return entity.ReactionBlock
	case value.ScanStatusSuspicious:
		return entity.ReactionReview
	default:
		return entity.ReactionBlock
	}
}
