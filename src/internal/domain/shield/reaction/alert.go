package reaction

import (
	"context"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 23-shield-reactions#T2.4: Implement AlertReaction (AC-004)
// @sk-task remove-audit-incidents#T2.1: Replace IncidentRepository with structured logging (AC-007)
type AlertReaction struct {
	log *zap.Logger
}

// @sk-task remove-audit-incidents#T2.1: Constructor without IncidentRepository (AC-007)
func NewAlertReaction(log *zap.Logger) *AlertReaction {
	return &AlertReaction{log: log}
}

// @sk-task remove-audit-incidents#T2.1: Log instead of creating incidents (AC-007)
func (r *AlertReaction) Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, nil
	}

	r.log.Info("shield alert",
		zap.String("status", string(result.Status())),
		zap.String("action", "alert"),
	)
	return text, nil
}
