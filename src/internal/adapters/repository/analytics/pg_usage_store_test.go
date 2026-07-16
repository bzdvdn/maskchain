package analyticsrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to test DB: %v", err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS usage_raw (
		id UUID PRIMARY KEY, tenant_id VARCHAR(255) NOT NULL,
		model VARCHAR(255) NOT NULL, input_tokens BIGINT NOT NULL CHECK (input_tokens >= 0),
		output_tokens BIGINT NOT NULL CHECK (output_tokens >= 0),
		cost NUMERIC(12,6) NOT NULL CHECK (cost >= 0),
		recorded_at TIMESTAMPTZ NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	return pool
}

func testTokenUsage(tid value.TenantID, model string, input, output int64, cost float64, ts time.Time) analytics.TokenUsage {
	return analytics.TokenUsage{
		TenantID:     tid,
		Model:        model,
		InputTokens:  input,
		OutputTokens: output,
		Cost:         cost,
		Timestamp:    ts,
	}
}

// @sk-test 131-analytics-pipeline#T4.6: TestPgUsageStoreRecord (AC-004, AC-008)
func TestPgUsageStoreRecord(t *testing.T) {
	pool := getTestPool(t)
	store := NewPgUsageStore(pool)
	ctx := context.Background()

	tid, _ := value.NewTenantID("test-tenant")
	now := time.Now().UTC()
	usage := testTokenUsage(tid, "gpt-4", 100, 50, 0.015, now)

	if err := store.Record(ctx, usage); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT tenant_id, model, input_tokens, output_tokens, cost FROM usage_raw WHERE tenant_id = $1`, tid.String())
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected at least 1 row")
	}
	var tid2, model string
	var input, output int64
	var cost float64
	if err := rows.Scan(&tid2, &model, &input, &output, &cost); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if tid2 != tid.String() {
		t.Errorf("tenant_id = %q, want %q", tid2, tid.String())
	}
	if model != "gpt-4" {
		t.Errorf("model = %q, want %q", model, "gpt-4")
	}
	if input != 100 {
		t.Errorf("input_tokens = %d, want %d", input, 100)
	}
	if output != 50 {
		t.Errorf("output_tokens = %d, want %d", output, 50)
	}
	_ = cost

	pool.Exec(ctx, `DELETE FROM usage_raw WHERE tenant_id = $1`, tid.String())
}

// @sk-test 131-analytics-pipeline#T4.6: TestPgUsageStoreRecordBatch (AC-004, AC-008)
func TestPgUsageStoreRecordBatch(t *testing.T) {
	pool := getTestPool(t)
	store := NewPgUsageStore(pool)
	ctx := context.Background()

	tid, _ := value.NewTenantID("test-tenant")
	now := time.Now().UTC()
	usages := make([]analytics.TokenUsage, 3)
	for i := range usages {
		usages[i] = testTokenUsage(tid, "gpt-4", int64(100*(i+1)), int64(50*(i+1)), float64(i+1)*0.01, now)
	}

	if err := store.RecordBatch(ctx, usages); err != nil {
		t.Fatalf("RecordBatch failed: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM usage_raw WHERE tenant_id = $1`, tid.String()).Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 records, got %d", count)
	}

	pool.Exec(ctx, `DELETE FROM usage_raw WHERE tenant_id = $1`, tid.String())
}

// @sk-test 131-analytics-pipeline#T4.6: TestPgUsageStoreDeleteOlderThan (AC-008)
func TestPgUsageStoreDeleteOlderThan(t *testing.T) {
	pool := getTestPool(t)
	store := NewPgUsageStore(pool)
	ctx := context.Background()

	tid, _ := value.NewTenantID("test-tenant")
	now := time.Now().UTC()

	oldUsage := testTokenUsage(tid, "gpt-4", 100, 50, 0.015, now.Add(-48*time.Hour))
	newUsage := testTokenUsage(tid, "gpt-4", 200, 100, 0.03, now)

	if err := store.Record(ctx, oldUsage); err != nil {
		t.Fatalf("Record old failed: %v", err)
	}
	if err := store.Record(ctx, newUsage); err != nil {
		t.Fatalf("Record new failed: %v", err)
	}

	cutoff := now.Add(-24 * time.Hour)
	deleted, err := store.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteOlderThan failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted record, got %d", deleted)
	}

	var remaining int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM usage_raw WHERE tenant_id = $1`, tid.String()).Scan(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining record, got %d", remaining)
	}

	pool.Exec(ctx, `DELETE FROM usage_raw WHERE tenant_id = $1`, tid.String())
}

// @sk-test 131-analytics-pipeline#T4.6: TestPgUsageStoreNilPool (AC-004, AC-008)
func TestPgUsageStoreNilPool(t *testing.T) {
	store := NewPgUsageStore(nil)
	ctx := context.Background()

	if err := store.Record(ctx, analytics.TokenUsage{}); err != nil {
		t.Errorf("expected no error with nil pool, got %v", err)
	}
	if err := store.RecordBatch(ctx, nil); err != nil {
		t.Errorf("expected no error with nil pool, got %v", err)
	}
	if _, err := store.DeleteOlderThan(ctx, time.Now()); err != nil {
		t.Errorf("expected no error with nil pool, got %v", err)
	}
}
