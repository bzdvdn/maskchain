package detector

import (
	"context"
	"regexp"
)

// @sk-task 21-shield-detectors#T3.3: Implement PHIDetector (AC-001, AC-007)
type PHIDetector struct {
	icd10 *regexp.Regexp
}

func NewPHIDetector() (*PHIDetector, error) {
	icd10, err := regexp.Compile(`\b[A-TV-Z][0-9][0-9AB]\.?[0-9A-Z]{0,2}\b`)
	if err != nil {
		return nil, err
	}
	return &PHIDetector{
		icd10: icd10,
	}, nil
}

func (d *PHIDetector) Scan(_ context.Context, text string) ([]DetectorResult, error) {
	results := make([]DetectorResult, 0)

	for _, m := range d.icd10.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "icd10",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}

	return results, nil
}
