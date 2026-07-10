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

func (e *PolicyEvaluator) Evaluate(result *entity.ScanResult) entity.Reaction {
	if result == nil {
		return entity.ReactionBlock
	}

	incidents := result.Incidents()
	if len(incidents) == 0 {
		return entity.ReactionAllow
	}

	highest := value.SeverityLow
	for _, inc := range incidents {
		if inc.Severity() > highest {
			highest = inc.Severity()
		}
	}

	switch highest {
	case value.SeverityCritical:
		return entity.ReactionBlock
	case value.SeverityHigh:
		return entity.ReactionReview
	case value.SeverityMedium, value.SeverityLow:
		return entity.ReactionLog
	default:
		return entity.ReactionBlock
	}
}
