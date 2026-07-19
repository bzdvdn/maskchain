package entity

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type Finding struct {
	DetectorType DetectorType
	Label        string
	Fragment     string
	StartPos     int
	EndPos       int
	Severity     value.Severity
}

// @sk-task 20-shield-domain#T2.3: Implement ScanResult entity
// @sk-task remove-audit-incidents#T1.2: Remove incidents field (AC-006)
type ScanResult struct {
	status    value.ScanStatus
	scannedAt time.Time
	findings  []Finding
}

// @sk-task remove-audit-incidents#T1.2: Adapt constructor without incidents (AC-006)
func NewScanResult(status value.ScanStatus) *ScanResult {
	return &ScanResult{
		status:    status,
		scannedAt: time.Now().UTC(),
	}
}

func NewScanResultWithFindings(status value.ScanStatus, findings []Finding) *ScanResult {
	return &ScanResult{
		status:    status,
		scannedAt: time.Now().UTC(),
		findings:  findings,
	}
}

func (r *ScanResult) Status() value.ScanStatus { return r.status }
func (r *ScanResult) ScannedAt() time.Time     { return r.scannedAt }
func (r *ScanResult) Findings() []Finding      { return r.findings }
