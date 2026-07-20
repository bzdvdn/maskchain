package shield

import (
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 50-shield-engine#T1.1: Create ScanRequest and ScanResponse DTOs
// @sk-task 13-shield-middleware-wiring#T1.2: Rules replaces ProfileSlug (AC-001)
//
// ScanRequest represents a domain entity or configuration.
type ScanRequest struct {
	Text  string
	Rules []entity.PIARule
}

// @sk-task 50-shield-engine#T1.1: Create ScanRequest and ScanResponse DTOs
//
// ScanResponse represents a domain entity or configuration.
type ScanResponse struct {
	*entity.ScanResult
	ProcessedText string
	Replacements  map[string]string
}
