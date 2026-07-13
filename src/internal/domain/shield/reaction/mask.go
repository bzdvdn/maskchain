package reaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-task 23-shield-reactions#T3.1: Implement MaskReaction (AC-003)
type MaskReaction struct {
	useCase *mask.MaskUseCase
}

func NewMaskReaction(useCase *mask.MaskUseCase) *MaskReaction {
	return &MaskReaction{useCase: useCase}
}

func (r *MaskReaction) Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, nil
	}

	incidents := result.Incidents()
	if len(incidents) == 0 {
		return text, nil
	}

	results := make([]detector.DetectorResult, 0, len(incidents))
	for _, inc := range incidents {
		fragment := inc.Fragment()
		if fragment == "" {
			continue
		}

		startPos := inc.Position()
		if startPos <= 0 {
			startPos = strings.Index(text, fragment)
			if startPos < 0 {
				continue
			}
		}
		endPos := startPos + len(fragment)
		if endPos > len(text) {
			endPos = len(text)
		}

		results = append(results, detector.DetectorResult{
			DetectorType: inc.DetectorID(),
			Fragment:     fragment,
			StartPos:     startPos,
			EndPos:       endPos,
			Confidence:   1.0,
		})
	}

	if len(results) == 0 {
		return text, nil
	}

	maskID := mask.NewShortID()
	maskedText, _, err := r.useCase.MaskFromResults(ctx, text, maskID, maskID, results)
	if err != nil {
		return text, fmt.Errorf("mask: %w", err)
	}
	return maskedText, nil
}
