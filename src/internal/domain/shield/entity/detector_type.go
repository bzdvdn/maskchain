package entity

// @sk-task 20-shield-domain#T2.2: Implement DetectorType enum (AC-008)
type DetectorType string

const (
	DetectorTypeRegex    DetectorType = "regex"
	DetectorTypeKeyword  DetectorType = "keyword"
	DetectorTypePresidio DetectorType = "presidio"
	// @sk-task 24-shield-dictionaries#T2.2: Add Dictionary DetectorType (AC-007)
	DetectorTypeDictionary DetectorType = "dictionary"
	// @sk-task prompt-injection-shield#T1.1: Add PromptInjection DetectorType (AC-001)
	DetectorTypePromptInjection DetectorType = "prompt_injection"
)
