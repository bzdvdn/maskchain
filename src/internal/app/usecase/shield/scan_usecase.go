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

// @sk-task remove-audit-incidents#T2.2: Remove incident creation from scan use case (AC-006)
func (uc *ScanUseCase) Scan(ctx context.Context, req ScanRequest) (*ScanResponse, error) {
	if len(req.Rules) == 0 {
		return &ScanResponse{
			ScanResult:    entity.NewScanResult(value.ScanStatusClean),
			ProcessedText: req.Text,
		}, nil
	}

	pipeline, err := uc.pipelineFactory.BuildFromRules(ctx, req.Rules)
	if err != nil {
		return nil, fmt.Errorf("build pipeline: %w", err)
	}

	blocked := false
	hasResults := false
	for _, binding := range pipeline.Detectors {
		results, err := binding.Interface.Scan(ctx, req.Text)
		if err != nil {
			continue
		}
		if len(results) > 0 {
			hasResults = true
		}
		if binding.Severity == value.SeverityCritical && len(results) > 0 {
			blocked = true
		}
	}

	var scanStatus value.ScanStatus
	switch {
	case blocked:
		scanStatus = value.ScanStatusBlocked
	case hasResults:
		scanStatus = value.ScanStatusSuspicious
	default:
		scanStatus = value.ScanStatusClean
	}

	scanResult := entity.NewScanResult(scanStatus)
	return &ScanResponse{
		ScanResult:    scanResult,
		ProcessedText: req.Text,
	}, nil
}
