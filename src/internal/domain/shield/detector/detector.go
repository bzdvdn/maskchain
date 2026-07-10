package detector

import "context"

// @sk-task 21-shield-detectors#T1.1: Define Detector interface (AC-001)
type Detector interface {
	Scan(ctx context.Context, text string) ([]DetectorResult, error)
}

// @sk-task 21-shield-detectors#T1.1: Define DetectorResult struct (AC-002)
type DetectorResult struct {
	DetectorType string
	Fragment     string
	StartPos     int
	EndPos       int
	Confidence   float64
}
