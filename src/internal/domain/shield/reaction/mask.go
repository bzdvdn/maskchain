package reaction

import (
	"context"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-task 23-shield-reactions#T3.1: Implement MaskReaction (AC-003)
// @sk-task remove-audit-incidents#T2.1: Remove Incident dependency, log instead (AC-007)
type MaskReaction struct {
	useCase *mask.MaskUseCase
	log     *zap.Logger
}

// @sk-task remove-audit-incidents#T2.1: Constructor without incidents (AC-007)
func NewMaskReaction(useCase *mask.MaskUseCase, log *zap.Logger) *MaskReaction {
	return &MaskReaction{useCase: useCase, log: log}
}

// @sk-task remove-audit-incidents#T2.1: Log instead of masking by incident data (AC-007)
func (r *MaskReaction) Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, nil
	}

	r.log.Info("shield mask reaction triggered",
		zap.String("status", string(result.Status())),
		zap.String("action", "mask"),
	)
	return text, nil
}
