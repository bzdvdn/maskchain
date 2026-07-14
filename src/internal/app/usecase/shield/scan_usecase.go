package shield

import (
	"context"
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 50-shield-engine#T2.1: Implement ScanUseCase orchestration (AC-001, AC-004, AC-005)
// @sk-task 50-shield-engine#T3.1: Consolidate replacements with placeholder format (AC-002, AC-006)
// @sk-task 13-shield-middleware-wiring#T1.3: Build pipeline from Rules instead of ProfileSlug (AC-001)
type ScanUseCase struct {
	pipelineFactory *ScanPipelineFactory
}

func NewScanUseCase(pipelineFactory *ScanPipelineFactory) *ScanUseCase {
	return &ScanUseCase{pipelineFactory: pipelineFactory}
}

func (uc *ScanUseCase) Scan(ctx context.Context, req ScanRequest) (*ScanResponse, error) {
	if len(req.Rules) == 0 {
		return &ScanResponse{
			ScanResult:    entity.NewScanResult(value.ScanStatusClean, nil),
			ProcessedText: req.Text,
		}, nil
	}

	pipeline, err := uc.pipelineFactory.BuildFromRules(ctx, req.Rules)
	if err != nil {
		return nil, fmt.Errorf("build pipeline: %w", err)
	}

	var incidents []entity.Incident
	for _, binding := range pipeline.Detectors {
		results, err := binding.Interface.Scan(ctx, req.Text)
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
		}
	}

	scanResult := entity.NewScanResult(statusFromIncidents(incidents), incidents)
	return &ScanResponse{
		ScanResult:    scanResult,
		ProcessedText: req.Text,
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
