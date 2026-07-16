package analytics

import (
	"context"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 130-analytics-domain#T2.2: Implement UsageStore port interface (AC-002)
type UsageStore interface {
	Record(ctx context.Context, usage TokenUsage) error
	QueryByTenant(ctx context.Context, tenantID value.TenantID, from, to time.Time) ([]UsageRecord, error)
	QueryByModel(ctx context.Context, model string, from, to time.Time) ([]UsageRecord, error)
	AggregateByDay(ctx context.Context, tenantID value.TenantID, from, to time.Time) ([]Aggregation, error)
}
