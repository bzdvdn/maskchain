package reaction

import (
	"context"
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 23-shield-reactions#T2.4: Implement AlertReaction (AC-004)
// @sk-task 60-audit-incidents#T4.1: AlertReaction uses repo.Save with PromptSnippetRedacted (AC-001)
type AlertReaction struct {
	repo shield.IncidentRepository
}

func NewAlertReaction(repo shield.IncidentRepository) *AlertReaction {
	return &AlertReaction{repo: repo}
}

func (r *AlertReaction) Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, nil
	}

	incidents := result.Incidents()
	for _, inc := range incidents {
		saveErr := r.repo.Save(ctx, &inc)
		if saveErr != nil {
			return text, fmt.Errorf("alert: save incident: %w", saveErr)
		}
	}
	return text, nil
}
