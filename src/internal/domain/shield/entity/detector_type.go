package entity

// @sk-task 20-shield-domain#T2.2: Implement DetectorType enum (AC-008)
type DetectorType string

const (
	DetectorTypeRegex    DetectorType = "regex"
	DetectorTypeKeyword  DetectorType = "keyword"
	DetectorTypePresidio DetectorType = "presidio"
)
