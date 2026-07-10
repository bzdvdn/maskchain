package reaction

import (
	"context"
	"errors"
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 23-shield-reactions#T2.5: Implement ReactionPipeline interface (AC-005, DEC-004)
type ReactionPipeline interface {
	Execute(ctx context.Context, reaction entity.Reaction, result *entity.ScanResult, text string) (string, error)
}

// @sk-task 23-shield-reactions#T2.5: Implement DefaultReactionPipeline (AC-005)
type DefaultReactionPipeline struct {
	blockExecutor  ReactionExecutor
	redactExecutor ReactionExecutor
	alertExecutor  ReactionExecutor
}

func NewDefaultReactionPipeline(block, redact, alert ReactionExecutor) *DefaultReactionPipeline {
	return &DefaultReactionPipeline{
		blockExecutor:  block,
		redactExecutor: redact,
		alertExecutor:  alert,
	}
}

func (p *DefaultReactionPipeline) Execute(ctx context.Context, reaction entity.Reaction, result *entity.ScanResult, text string) (string, error) {
	var exec ReactionExecutor
	switch reaction {
	case entity.ReactionBlock:
		exec = p.blockExecutor
	case entity.ReactionLog:
		exec = p.redactExecutor
	case entity.ReactionReview:
		exec = p.alertExecutor
	case entity.ReactionAllow:
		return text, nil
	default:
		return text, fmt.Errorf("unknown reaction: %s", reaction)
	}

	if exec == nil {
		return text, errors.New("reaction executor not configured")
	}
	return exec.Execute(ctx, result, text)
}
