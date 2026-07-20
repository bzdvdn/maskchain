package detector

import (
	"context"
	"regexp"
)

// @sk-task 21-shield-detectors#T3.1: Implement SecretsDetector (AC-001, AC-004)
//
// SecretsDetector represents a domain entity or configuration.
type SecretsDetector struct {
	apiKey     *regexp.Regexp
	jwt        *regexp.Regexp
	privateKey *regexp.Regexp
}

func NewSecretsDetector() (*SecretsDetector, error) {
	apiKey, err := regexp.Compile(`\b(?:sk\-|pk\-|bearer\s+)[a-zA-Z0-9_\-]{16,}\b`)
	if err != nil {
		return nil, err
	}
	jwt, err := regexp.Compile(`eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+`)
	if err != nil {
		return nil, err
	}
	privateKey, err := regexp.Compile(`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(?:RSA\s+)?PRIVATE\s+KEY-----`)
	if err != nil {
		return nil, err
	}
	return &SecretsDetector{
		apiKey:     apiKey,
		jwt:        jwt,
		privateKey: privateKey,
	}, nil
}

func (d *SecretsDetector) Scan(_ context.Context, text string) ([]DetectorResult, error) {
	results := make([]DetectorResult, 0)

	for _, m := range d.apiKey.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "api_key",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}
	for _, m := range d.jwt.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "jwt",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}
	for _, m := range d.privateKey.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "private_key",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}

	return results, nil
}
