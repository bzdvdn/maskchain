package analytics

import (
	"context"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 130-analytics-domain#T2.2: Implement UsageStore port interface (AC-002)
// @sk-task 131-analytics-pipeline#T2.1: Add RecordBatch and DeleteOlderThan to UsageStore (AC-002, AC-008)
type UsageStore interface {
	Record(ctx context.Context, usage TokenUsage) error
	RecordBatch(ctx context.Context, usages []TokenUsage) error
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
	QueryByTenant(ctx context.Context, tenantID value.TenantID, from, to time.Time) ([]UsageRecord, error)
	QueryByModel(ctx context.Context, model string, from, to time.Time) ([]UsageRecord, error)
	QueryAll(ctx context.Context, from, to time.Time) ([]UsageRecord, error)
	AggregateByDay(ctx context.Context, tenantID value.TenantID, from, to time.Time) ([]Aggregation, error)
	// @sk-task timeseries-grafana#T1.2: QueryTimeSeries returns bucketed aggregation
	QueryTimeSeries(ctx context.Context, from, to time.Time) ([]TimeSeriesPoint, error)
}
