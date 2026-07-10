package detector

import "context"

// @sk-task 22-shield-mask-storage#T1.2: Implement CompositeDetector (AC-011)
type CompositeDetector struct {
	detectors []Detector
}

func NewCompositeDetector(detectors ...Detector) *CompositeDetector {
	return &CompositeDetector{detectors: detectors}
}

// @sk-task 22-shield-mask-storage#T1.2: CompositeDetector.Scan merges results (AC-011)
func (d *CompositeDetector) Scan(ctx context.Context, text string) ([]DetectorResult, error) {
	var results []DetectorResult
	for _, det := range d.detectors {
		r, err := det.Scan(ctx, text)
		if err != nil {
			return nil, err
		}
		results = append(results, r...)
	}
	return results, nil
}
