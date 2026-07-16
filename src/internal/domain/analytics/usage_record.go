package analytics

import "time"

// @sk-task 130-analytics-domain#T2.1: Implement UsageRecord value object (AC-005)
type UsageRecord struct {
	TenantID          string
	Model             string
	PeriodStart       time.Time
	PeriodEnd         time.Time
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCost         float64
	RequestCount      int64
}

func NewUsageRecord(tenantID, model string, periodStart, periodEnd time.Time, totalInputTokens, totalOutputTokens int64, totalCost float64, requestCount int64) *UsageRecord {
	return &UsageRecord{
		TenantID:          tenantID,
		Model:             model,
		PeriodStart:       periodStart,
		PeriodEnd:         periodEnd,
		TotalInputTokens:  totalInputTokens,
		TotalOutputTokens: totalOutputTokens,
		TotalCost:         totalCost,
		RequestCount:      requestCount,
	}
}
