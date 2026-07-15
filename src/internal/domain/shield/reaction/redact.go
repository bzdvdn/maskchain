package reaction

import (
	"context"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 23-shield-reactions#T2.3: Implement RedactReaction (AC-002)
// @sk-task remove-audit-incidents#T2.1: Remove Incident dependency, log instead (AC-007)
type RedactReaction struct {
	log *zap.Logger
}

// @sk-task remove-audit-incidents#T2.1: Constructor with logger instead of incidents (AC-007)
func NewRedactReaction(log *zap.Logger) *RedactReaction {
	return &RedactReaction{log: log}
}

// @sk-task remove-audit-incidents#T2.1: Log instead of redacting by incident data (AC-007)
func (r *RedactReaction) Execute(_ context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, nil
	}

	r.log.Info("shield redact reaction triggered",
		zap.String("status", string(result.Status())),
		zap.String("action", "redact"),
	)
	return text, nil
}
