package analytics

import (
	"fmt"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 130-analytics-domain#T1.2: Implement TokenUsage entity (AC-001)
type TokenUsage struct {
	TenantID      value.TenantID
	Model         string
	InputTokens   int64
	OutputTokens  int64
	Cost          float64
	Timestamp     time.Time
}

func NewTokenUsage(tenantID value.TenantID, model string, inputTokens, outputTokens int64, cost float64, timestamp time.Time) (*TokenUsage, error) {
	if tenantID.String() == "" {
		return nil, fmt.Errorf("tenantID must not be empty")
	}
	if model == "" {
		return nil, fmt.Errorf("model must not be empty")
	}
	if inputTokens < 0 {
		return nil, fmt.Errorf("inputTokens must not be negative")
	}
	if outputTokens < 0 {
		return nil, fmt.Errorf("outputTokens must not be negative")
	}
	if cost < 0 {
		return nil, fmt.Errorf("cost must not be negative")
	}
	if timestamp.IsZero() {
		return nil, fmt.Errorf("timestamp must not be zero")
	}
	return &TokenUsage{
		TenantID:     tenantID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Timestamp:    timestamp,
	}, nil
}
