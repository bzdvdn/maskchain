package value

// @sk-task 20-shield-domain#T1.1: Implement ScanStatus value object (AC-006)
//
// ScanStatus is a string type for domain values.
type ScanStatus string

const (
	ScanStatusClean      ScanStatus = "clean"
	ScanStatusSuspicious ScanStatus = "suspicious"
	ScanStatusBlocked    ScanStatus = "blocked"
	ScanStatusError      ScanStatus = "error"
)
