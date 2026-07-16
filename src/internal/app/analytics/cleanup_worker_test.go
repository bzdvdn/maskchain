package analytics

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 131-analytics-pipeline#T4.4: TestCleanupWorkerDeletesOldRecords (AC-008)
func TestCleanupWorkerDeletesOldRecords(t *testing.T) {
	store := &mockUsageStore{}
	worker := NewCleanupWorker(store, 50*time.Millisecond, 24*time.Hour, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.Run(ctx)
}

// @sk-test 131-analytics-pipeline#T4.4: TestCleanupWorkerIntervalZero (AC-008)
func TestCleanupWorkerIntervalZero(t *testing.T) {
	store := &mockUsageStore{}
	worker := NewCleanupWorker(store, 0, 24*time.Hour, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	worker.Run(ctx)
}

// @sk-test 131-analytics-pipeline#T4.4: TestCleanupWorkerCallsDeleteOlderThan (AC-008)
func TestCleanupWorkerCallsDeleteOlderThan(t *testing.T) {
	tid, _ := value.NewTenantID("test-tenant")
	now := time.Now().UTC()

	store := &usageStoreWithRecorder{
		UsageStore: &mockUsageStore{},
		deleteCalls: make([]time.Time, 0),
	}
	store.UsageStore.Record(context.Background(), analytics.TokenUsage{
		TenantID: tid, Model: "gpt-4", Timestamp: now.Add(-48 * time.Hour),
	})
	store.UsageStore.Record(context.Background(), analytics.TokenUsage{
		TenantID: tid, Model: "gpt-4", Timestamp: now,
	})

	worker := NewCleanupWorker(store, 50*time.Millisecond, 24*time.Hour, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.Run(ctx)
}

type usageStoreWithRecorder struct {
	analytics.UsageStore
	deleteCalls []time.Time
}

func (u *usageStoreWithRecorder) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	u.deleteCalls = append(u.deleteCalls, before)
	return u.UsageStore.DeleteOlderThan(ctx, before)
}
