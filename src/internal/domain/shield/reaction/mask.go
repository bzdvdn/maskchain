package reaction

import (
	"context"
	"log/slog"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-task 23-shield-reactions#T3.1: Implement MaskReaction (AC-003)
// @sk-task remove-audit-incidents#T2.1: Remove Incident dependency, log instead (AC-007)
//
// MaskReaction represents a domain entity or configuration.
type MaskReaction struct {
	useCase *mask.MaskUseCase
	log     *slog.Logger
}

// @sk-task remove-audit-incidents#T2.1: Constructor without incidents (AC-007)
//
// NewMaskReaction creates a new MaskReaction.
func NewMaskReaction(useCase *mask.MaskUseCase, log *slog.Logger) *MaskReaction {
	return &MaskReaction{useCase: useCase, log: log}
}

// @sk-task remove-audit-incidents#T2.1: Log instead of masking by incident data (AC-007)
func (r *MaskReaction) Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, nil
	}

	r.log.InfoContext(ctx, "shield mask reaction triggered",
		slog.String("status", string(result.Status())),
		slog.String("action", "mask"),
	)
	return text, nil
}
