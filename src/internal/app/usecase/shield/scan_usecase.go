package shield

import (
	"context"
	"fmt"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/reaction"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 50-shield-engine#T2.1: Implement ScanUseCase orchestration (AC-001, AC-004, AC-005)
// @sk-task 50-shield-engine#T3.1: Consolidate replacements with placeholder format (AC-002, AC-006)
type ScanUseCase struct {
	profileRepo      shield.ProfileRepository
	pipelineFactory  *ScanPipelineFactory
	policyEvaluator  *service.PolicyEvaluator
	reactionPipeline reaction.ReactionPipeline
	tenantID         value.TenantID
	maskMode         bool
}

type ScanUseCaseOption func(*ScanUseCase)

func WithMaskMode() ScanUseCaseOption {
	return func(uc *ScanUseCase) { uc.maskMode = true }
}

func NewScanUseCase(
	profileRepo shield.ProfileRepository,
	pipelineFactory *ScanPipelineFactory,
	policyEvaluator *service.PolicyEvaluator,
	reactionPipeline reaction.ReactionPipeline,
	tenantID value.TenantID,
	opts ...ScanUseCaseOption,
) *ScanUseCase {
	uc := &ScanUseCase{
		profileRepo:      profileRepo,
		pipelineFactory:  pipelineFactory,
		policyEvaluator:  policyEvaluator,
		reactionPipeline: reactionPipeline,
		tenantID:         tenantID,
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

func (uc *ScanUseCase) Scan(ctx context.Context, req ScanRequest) (*ScanResponse, error) {
	slug, err := value.NewProfileSlug(req.ProfileSlug)
	if err != nil {
		return nil, fmt.Errorf("invalid profile slug: %w", err)
	}

	profile, err := uc.profileRepo.FindBySlug(ctx, uc.tenantID, slug)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrProfileNotFound, req.ProfileSlug)
	}

	if !profile.Enabled() {
		return nil, fmt.Errorf("%w: %s", ErrProfileDisabled, req.ProfileSlug)
	}

	pipeline, err := uc.pipelineFactory.Build(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("build pipeline: %w", err)
	}

	if len(pipeline.Preprocessors) == 0 && len(pipeline.Detectors) == 0 {
		return &ScanResponse{
			ScanResult:    entity.NewScanResult(value.ScanStatusClean, nil),
			ProcessedText: req.Text,
			Replacements:  nil,
		}, nil
	}

	currentText := req.Text
	allReplacements := make(map[string]string)

	for _, p := range pipeline.Preprocessors {
		res := p.Process(currentText, "default")
		if res == nil {
			continue
		}
		currentText = res.ModifiedText
		for orig, ph := range res.Replacements {
			allReplacements[ph] = orig
		}
	}

	var incidents []entity.Incident
	type phEntry struct{ orig, ph string }
	var phEntries []phEntry
	phCounter := 0

	for _, binding := range pipeline.Detectors {
		results, err := binding.Interface.Scan(ctx, currentText)
		if err != nil {
			continue
		}
		for _, r := range results {
			inc := entity.NewIncident(
				binding.Label,
				value.PatternID{},
				binding.Severity,
				r.Fragment,
				r.StartPos,
			)
			incidents = append(incidents, *inc)

			if uc.maskMode {
				prefix := "p"
				if strings.HasPrefix(binding.Label, "dictionary:") {
					prefix = "dict"
				}
				ph := fmt.Sprintf("{{%s.default.%d}}", prefix, phCounter)
				phCounter++
				phEntries = append(phEntries, phEntry{orig: r.Fragment, ph: ph})
			}
		}
	}

	scanResult := entity.NewScanResult(statusFromIncidents(incidents), incidents)

	if uc.maskMode && len(phEntries) > 0 {
		for _, e := range phEntries {
			currentText = strings.Replace(currentText, e.orig, e.ph, 1)
			allReplacements[e.ph] = e.orig
		}
		return &ScanResponse{
			ScanResult:    scanResult,
			ProcessedText: currentText,
			Replacements:  allReplacements,
		}, nil
	}

	r := uc.policyEvaluator.Evaluate(scanResult)

	processedText, err := uc.reactionPipeline.Execute(ctx, r, scanResult, currentText)
	if err != nil {
		return nil, fmt.Errorf("execute reaction: %w", err)
	}

	return &ScanResponse{
		ScanResult:    scanResult,
		ProcessedText: processedText,
		Replacements:  allReplacements,
	}, nil
}

func statusFromIncidents(incidents []entity.Incident) value.ScanStatus {
	if len(incidents) == 0 {
		return value.ScanStatusClean
	}
	blocked := false
	for _, inc := range incidents {
		if inc.Severity() == value.SeverityCritical {
			blocked = true
			break
		}
	}
	if blocked {
		return value.ScanStatusBlocked
	}
	return value.ScanStatusSuspicious
}
