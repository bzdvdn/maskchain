package analytics

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type mockUsageStore struct {
	mu           sync.Mutex
	recordBatchCalls int
	batches          [][]analytics.TokenUsage
}

func (m *mockUsageStore) Record(_ context.Context, _ analytics.TokenUsage) error {
	return nil
}

func (m *mockUsageStore) RecordBatch(_ context.Context, usages []analytics.TokenUsage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordBatchCalls++
	batch := make([]analytics.TokenUsage, len(usages))
	copy(batch, usages)
	m.batches = append(m.batches, batch)
	return nil
}

func (m *mockUsageStore) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (m *mockUsageStore) QueryByTenant(_ context.Context, _ value.TenantID, _, _ time.Time) ([]analytics.UsageRecord, error) {
	return nil, nil
}

func (m *mockUsageStore) QueryByModel(_ context.Context, _ string, _, _ time.Time) ([]analytics.UsageRecord, error) {
	return nil, nil
}

func (m *mockUsageStore) QueryAll(_ context.Context, _, _ time.Time) ([]analytics.UsageRecord, error) {
	return nil, nil
}

func (m *mockUsageStore) AggregateByDay(_ context.Context, _ value.TenantID, _, _ time.Time) ([]analytics.Aggregation, error) {
	return nil, nil
}

func testTokenUsage() analytics.TokenUsage {
	tid, _ := value.NewTenantID("test-tenant")
	return analytics.TokenUsage{
		TenantID:     tid,
		Model:        "gpt-4",
		InputTokens:  100,
		OutputTokens: 50,
		Cost:         0.015,
		Timestamp:    time.Now(),
	}
}

// @sk-test 131-analytics-pipeline#T4.2: TestAsyncWorkerBatchInsert (AC-002)
func TestAsyncWorkerBatchInsert(t *testing.T) {
	store := &mockUsageStore{}
	worker := NewAsyncWorker(store, 1000, 50*time.Millisecond, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go worker.Run(ctx)

	for i := 0; i < 10; i++ {
		worker.Send(testTokenUsage())
	}

	<-ctx.Done()

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.recordBatchCalls == 0 {
		t.Fatal("expected at least 1 RecordBatch call")
	}

	var totalRecords int
	for _, b := range store.batches {
		totalRecords += len(b)
	}
	if totalRecords < 10 {
		t.Errorf("expected at least 10 records in batch(es), got %d", totalRecords)
	}
}

// @sk-test 131-analytics-pipeline#T4.2: TestAsyncWorkerBufferOverflow (AC-002)
func TestAsyncWorkerBufferOverflow(t *testing.T) {
	store := &mockUsageStore{}
	worker := NewAsyncWorker(store, 5, time.Hour, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go worker.Run(ctx)

	for i := 0; i < 10; i++ {
		worker.Send(testTokenUsage())
	}

	<-ctx.Done()

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.recordBatchCalls == 0 {
		t.Error("expected RecordBatch to be called (sync flush on buffer overflow)")
	}
}

// @sk-test 131-analytics-pipeline#T4.2: TestAsyncWorkerGracefulShutdown (AC-002)
func TestAsyncWorkerGracefulShutdown(t *testing.T) {
	store := &mockUsageStore{}
	worker := NewAsyncWorker(store, 1000, time.Hour, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	worker.Send(testTokenUsage())
	worker.Send(testTokenUsage())

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.recordBatchCalls != 1 {
		t.Errorf("expected 1 RecordBatch call on shutdown, got %d", store.recordBatchCalls)
	}
}

// @sk-test 131-analytics-pipeline#T4.2: TestAsyncWorkerBufferNonBlockingSend (AC-002)
func TestAsyncWorkerBufferNonBlockingSend(t *testing.T) {
	store := &mockUsageStore{}
	worker := NewAsyncWorker(store, 0, time.Hour, zap.NewNop())

	worker.Send(testTokenUsage())
}
