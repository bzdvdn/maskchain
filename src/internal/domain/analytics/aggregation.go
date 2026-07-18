package analytics

import (
	"time"
)

// @sk-task 130-analytics-domain#T2.1: Implement Aggregation entity (AC-003)
type Aggregation struct {
	TenantID     string
	Model        string
	Date         time.Time
	TotalTokens  int64
	TotalCost    float64
	RequestCount int64
	AvgLatency   time.Duration
}

// @sk-task timeseries-grafana#T1.1: TimeSeriesPoint for bucketed time-series data
type TimeSeriesPoint struct {
	Bucket       time.Time
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	Requests     int64
}

func NewAggregation(tenantID, model string, date time.Time, totalTokens int64, totalCost float64, requestCount int64, avgLatency time.Duration) *Aggregation {
	return &Aggregation{
		TenantID:     tenantID,
		Model:        model,
		Date:         date,
		TotalTokens:  totalTokens,
		TotalCost:    totalCost,
		RequestCount: requestCount,
		AvgLatency:   avgLatency,
	}
}
