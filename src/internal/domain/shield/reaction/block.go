package reaction

import (
	"context"
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
)

// @sk-task 23-shield-reactions#T2.2: Implement BlockReaction (AC-001, DEC-003)
type BlockReaction struct{}

func NewBlockReaction() *BlockReaction {
	return &BlockReaction{}
}

func (r *BlockReaction) Execute(_ context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, fmt.Errorf("%w: nil scan result", shielderrors.ErrBlockedByPolicy)
	}

	incidents := result.Incidents()
	if len(incidents) == 0 {
		return text, fmt.Errorf("%w: no incidents found", shielderrors.ErrBlockedByPolicy)
	}

	inc := incidents[0]
	return text, fmt.Errorf("%w: blocked by detector %q with severity %v",
		shielderrors.ErrBlockedByPolicy, inc.DetectorID(), inc.Severity())
}
