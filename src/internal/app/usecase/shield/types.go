package shield

import (
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 50-shield-engine#T1.1: Create ScanRequest and ScanResponse DTOs
type ScanRequest struct {
	Text        string
	ProfileSlug string
}

// @sk-task 50-shield-engine#T1.1: Create ScanRequest and ScanResponse DTOs
type ScanResponse struct {
	*entity.ScanResult
	ProcessedText string
	Replacements  map[string]string
}
