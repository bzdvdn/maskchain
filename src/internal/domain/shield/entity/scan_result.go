package entity

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T2.3: Implement ScanResult entity
// @sk-task remove-audit-incidents#T1.2: Remove incidents field (AC-006)
type ScanResult struct {
	status    value.ScanStatus
	scannedAt time.Time
}

// @sk-task remove-audit-incidents#T1.2: Adapt constructor without incidents (AC-006)
func NewScanResult(status value.ScanStatus) *ScanResult {
	return &ScanResult{
		status:    status,
		scannedAt: time.Now().UTC(),
	}
}

func (r *ScanResult) Status() value.ScanStatus  { return r.status }
func (r *ScanResult) ScannedAt() time.Time      { return r.scannedAt }
