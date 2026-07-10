package entity

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T2.3: Implement ScanResult entity
type ScanResult struct {
	status    value.ScanStatus
	incidents []Incident
	scannedAt time.Time
}

func NewScanResult(status value.ScanStatus, incidents []Incident) *ScanResult {
	return &ScanResult{
		status:    status,
		incidents: incidents,
		scannedAt: time.Now().UTC(),
	}
}

func (r *ScanResult) Status() value.ScanStatus  { return r.status }
func (r *ScanResult) Incidents() []Incident     { return r.incidents }
func (r *ScanResult) ScannedAt() time.Time      { return r.scannedAt }
