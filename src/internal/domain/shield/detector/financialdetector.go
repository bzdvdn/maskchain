package detector

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// @sk-task 21-shield-detectors#T3.2: Implement FinancialDetector (AC-001, AC-005, AC-006)
type FinancialDetector struct {
	cardRegex  *regexp.Regexp
	ibanRegex  *regexp.Regexp
	swiftRegex *regexp.Regexp
}

func NewFinancialDetector() (*FinancialDetector, error) {
	card, err := regexp.Compile(`\b\d{13,19}\b`)
	if err != nil {
		return nil, err
	}
	iban, err := regexp.Compile(`\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b`)
	if err != nil {
		return nil, err
	}
	swift, err := regexp.Compile(`\b[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}(?:[A-Z0-9]{3})?\b`)
	if err != nil {
		return nil, err
	}
	return &FinancialDetector{
		cardRegex:  card,
		ibanRegex:  iban,
		swiftRegex: swift,
	}, nil
}

func (d *FinancialDetector) Scan(_ context.Context, text string) ([]DetectorResult, error) {
	results := make([]DetectorResult, 0)

	for _, m := range d.cardRegex.FindAllStringIndex(text, -1) {
		digits := stripNonDigits(text[m[0]:m[1]])
		if len(digits) >= 13 && len(digits) <= 19 && validLuhn(digits) {
			results = append(results, DetectorResult{
				DetectorType: "credit_card",
				Fragment:     text[m[0]:m[1]],
				StartPos:     m[0],
				EndPos:       m[1],
				Confidence:   1.0,
			})
		}
	}
	for _, m := range d.ibanRegex.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "iban",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}
	for _, m := range d.swiftRegex.FindAllStringIndex(text, -1) {
		results = append(results, DetectorResult{
			DetectorType: "swift",
			Fragment:     text[m[0]:m[1]],
			StartPos:     m[0],
			EndPos:       m[1],
			Confidence:   1.0,
		})
	}

	return results, nil
}

func stripNonDigits(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func validLuhn(s string) bool {
	if len(s) == 0 {
		return false
	}
	sum := 0
	alternate := false
	for i := len(s) - 1; i >= 0; i-- {
		digit, err := strconv.Atoi(string(s[i]))
		if err != nil {
			return false
		}
		if alternate {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		alternate = !alternate
	}
	return sum%10 == 0
}
