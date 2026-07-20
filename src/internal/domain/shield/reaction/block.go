package reaction

import (
	"context"
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
)

// @sk-task 23-shield-reactions#T2.2: Implement BlockReaction (AC-001, DEC-003)
//
// BlockReaction represents a domain entity or configuration.
type BlockReaction struct{}

func NewBlockReaction() *BlockReaction {
	return &BlockReaction{}
}

// @sk-task remove-audit-incidents#T2.1: Remove incidents reference, use status (AC-007)
func (r *BlockReaction) Execute(_ context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, fmt.Errorf("%w: nil scan result", shielderrors.ErrBlockedByPolicy)
	}

	return text, fmt.Errorf("%w: blocked with status %v",
		shielderrors.ErrBlockedByPolicy, result.Status())
}
