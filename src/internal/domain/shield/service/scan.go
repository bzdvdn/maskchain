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

// @sk-task remove-audit-incidents#T1.4: Remove incident creation from scan execution (AC-006)
func (p *ScanPipeline) Execute(detectors []entity.Detector, content string) *entity.ScanResult {
	if len(detectors) == 0 {
		return entity.NewScanResult(value.ScanStatusClean)
	}

	blocked := false
	suspicious := false

	for _, det := range detectors {
		if !det.Enabled() {
			continue
		}
		for _, pat := range det.Patterns() {
			if matchContent(content, pat.Expression()) {
				if det.Severity() == value.SeverityCritical {
					blocked = true
				} else {
					suspicious = true
				}
			}
		}
	}

	if blocked {
		return entity.NewScanResult(value.ScanStatusBlocked)
	}
	if suspicious {
		return entity.NewScanResult(value.ScanStatusSuspicious)
	}
	return entity.NewScanResult(value.ScanStatusClean)
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
