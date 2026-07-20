package reaction

import (
	"context"
	"log/slog"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 23-shield-reactions#T2.3: Implement RedactReaction (AC-002)
// @sk-task remove-audit-incidents#T2.1: Remove Incident dependency, log instead (AC-007)
//
// RedactReaction represents a domain entity or configuration.
type RedactReaction struct {
	log *slog.Logger
}

// @sk-task remove-audit-incidents#T2.1: Constructor with logger instead of incidents (AC-007)
//
// NewRedactReaction creates a new RedactReaction.
func NewRedactReaction(log *slog.Logger) *RedactReaction {
	return &RedactReaction{log: log}
}

// @sk-task remove-audit-incidents#T2.1: Log instead of redacting by incident data (AC-007)
func (r *RedactReaction) Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, nil
	}

	r.log.InfoContext(ctx, "shield redact reaction triggered",
		slog.String("status", string(result.Status())),
		slog.String("action", "redact"),
	)
	return text, nil
}
