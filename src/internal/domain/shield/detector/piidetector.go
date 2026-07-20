package detector

import (
	"context"
	"regexp"
)

// @sk-task 21-shield-detectors#T2.1: Implement PIIDetector (AC-001, AC-003)
//
// PIIDetector represents a domain entity or configuration.
type PIIDetector struct {
	email    *regexp.Regexp
	phone    *regexp.Regexp
	ssn      *regexp.Regexp
	passport *regexp.Regexp
}

func NewPIIDetector() (*PIIDetector, error) {
	email, err := regexp.Compile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	if err != nil {
		return nil, err
	}
	phone, err := regexp.Compile(`\+\d{1,3}[\s.\-]?\(?\d{1,4}\)?[\s.\-]?\d{1,4}[\s.\-]?\d{1,4}[\s.\-]?\d{1,9}`)
	if err != nil {
		return nil, err
	}
	ssn, err := regexp.Compile(`\b\d{3}-\d{2}-\d{4}\b`)
	if err != nil {
		return nil, err
	}
	passport, err := regexp.Compile(`\b\d{4}\s?\d{6}\b`)
	if err != nil {
		return nil, err
	}
	return &PIIDetector{
		email:    email,
		phone:    phone,
		ssn:      ssn,
		passport: passport,
	}, nil
}

func (d *PIIDetector) Scan(_ context.Context, text string) ([]DetectorResult, error) {
	results := make([]DetectorResult, 0)

	for _, m := range d.email.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "email",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}
	for _, m := range d.phone.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "phone",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}
	for _, m := range d.ssn.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "ssn",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}
	for _, m := range d.passport.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "passport",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}

	return results, nil
}
