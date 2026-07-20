package shield

import (
	"context"
	"fmt"
	"sort"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 50-shield-engine#T2.1: Implement ScanUseCase orchestration (AC-001, AC-004, AC-005)
// @sk-task 50-shield-engine#T3.1: Consolidate replacements with placeholder format (AC-002, AC-006)
// @sk-task 13-shield-middleware-wiring#T1.3: Build pipeline from Rules instead of ProfileSlug (AC-001)
//
// ScanUseCase represents a domain entity or configuration.
type ScanUseCase struct {
	pipelineFactory *ScanPipelineFactory
}

func NewScanUseCase(pipelineFactory *ScanPipelineFactory) *ScanUseCase {
	return &ScanUseCase{pipelineFactory: pipelineFactory}
}

type scanHit struct {
	label    string
	fragment string
	startPos int
	endPos   int
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
	var hits []scanHit
	var findings []entity.Finding
	for _, binding := range pipeline.Detectors {
		results, err := binding.Interface.Scan(ctx, req.Text)
		if err != nil {
			continue
		}
		if len(results) > 0 {
			for _, r := range results {
				hits = append(hits, scanHit{
					label:    binding.Label,
					fragment: r.Fragment,
					startPos: r.StartPos,
					endPos:   r.EndPos,
				})
				findings = append(findings, entity.Finding{
					DetectorType: binding.Type,
					Label:        binding.Label,
					Fragment:     r.Fragment,
					StartPos:     r.StartPos,
					EndPos:       r.EndPos,
					Severity:     binding.Severity,
				})
			}
		}
		if binding.Severity == value.SeverityCritical && len(results) > 0 {
			blocked = true
		}
	}

	var scanStatus value.ScanStatus
	switch {
	case blocked:
		scanStatus = value.ScanStatusBlocked
	case len(hits) > 0:
		scanStatus = value.ScanStatusSuspicious
	default:
		scanStatus = value.ScanStatusClean
	}

	replacements := make(map[string]string)
	processedText := req.Text
	if len(hits) > 0 {
		sort.Slice(hits, func(i, j int) bool {
			return hits[i].startPos > hits[j].startPos
		})
		labelCounters := make(map[string]int)
		for _, h := range hits {
			labelCounters[h.label]++
			ph := fmt.Sprintf("[[pii.%s.%d]]", h.label, labelCounters[h.label]-1)
			replacements[ph] = h.fragment
			processedText = processedText[:h.startPos] + ph + processedText[h.endPos:]
		}
	}

	scanResult := entity.NewScanResultWithFindings(scanStatus, findings)
	return &ScanResponse{
		ScanResult:    scanResult,
		ProcessedText: processedText,
		Replacements:  replacements,
	}, nil
}
