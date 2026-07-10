package service

import (
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T4.1: Implement ScanPipeline service (AC-003)
type ScanPipeline struct{}

func NewScanPipeline() *ScanPipeline {
	return &ScanPipeline{}
}

func (p *ScanPipeline) Execute(detectors []entity.Detector, content string) *entity.ScanResult {
	if len(detectors) == 0 {
		return entity.NewScanResult(value.ScanStatusClean, nil)
	}

	var incidents []entity.Incident
	blocked := false

	for _, det := range detectors {
		if !det.Enabled() {
			continue
		}
		// For each enabled detector, check each pattern
		for _, pat := range det.Patterns() {
			if matchContent(content, pat.Expression()) {
				incidents = append(incidents, *entity.NewIncident(
					det.ID(),
					pat.ID(),
					det.Severity(),
					pat.Expression(),
					0,
				))
				if det.Severity() == value.SeverityCritical {
					blocked = true
				}
			}
		}
	}

	if blocked {
		return entity.NewScanResult(value.ScanStatusBlocked, incidents)
	}
	if len(incidents) > 0 {
		return entity.NewScanResult(value.ScanStatusSuspicious, incidents)
	}
	return entity.NewScanResult(value.ScanStatusClean, nil)
}

func matchContent(content string, expr string) bool {
	return expr != "" && contains(content, expr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
