package analytics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
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

	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS usage_raw (
			id UUID PRIMARY KEY, tenant_id VARCHAR(255) NOT NULL,
			model VARCHAR(255) NOT NULL, input_tokens BIGINT NOT NULL,
			output_tokens BIGINT NOT NULL, cost NUMERIC(12,6) NOT NULL,
			recorded_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS usage_agg_hourly (
			tenant_id VARCHAR(255) NOT NULL, model VARCHAR(255) NOT NULL,
			hour TIMESTAMPTZ NOT NULL,
			total_input_tokens BIGINT NOT NULL, total_output_tokens BIGINT NOT NULL,
			total_cost NUMERIC(14,6) NOT NULL, request_count BIGINT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (tenant_id, model, hour)
		)`,
		`CREATE TABLE IF NOT EXISTS usage_agg_daily (
			tenant_id VARCHAR(255) NOT NULL, model VARCHAR(255) NOT NULL,
			day DATE NOT NULL,
			total_input_tokens BIGINT NOT NULL, total_output_tokens BIGINT NOT NULL,
			total_cost NUMERIC(14,6) NOT NULL, request_count BIGINT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (tenant_id, model, day)
		)`,
	} {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatalf("failed to create test tables: %v", err)
		}
	}
	return pool
}

// @sk-test 131-analytics-pipeline#T4.3: TestAggregationWorkerHourly (AC-004)
func TestAggregationWorkerHourly(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	now := time.Now().UTC()
	hourBoundary := now.Truncate(time.Hour)

	pool.Exec(ctx, `INSERT INTO usage_raw (id, tenant_id, model, input_tokens, output_tokens, cost, recorded_at) VALUES
		(gen_random_uuid(), 't1', 'gpt-4', 100, 50, 0.015, $1),
		(gen_random_uuid(), 't1', 'gpt-4', 200, 100, 0.03, $1)`, hourBoundary)

	worker := NewAggregationWorker(pool, time.Hour, zap.NewNop())
	worker.materializeHourly(ctx, hourBoundary)

	var rowCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM usage_agg_hourly WHERE tenant_id = 't1' AND model = 'gpt-4'`).Scan(&rowCount)
	if rowCount != 1 {
		t.Fatalf("expected 1 hourly agg row, got %d", rowCount)
	}

	var totalInput, totalOutput int64
	var totalCost float64
	var reqCount int64
	pool.QueryRow(ctx, `SELECT total_input_tokens, total_output_tokens, total_cost, request_count
		FROM usage_agg_hourly WHERE tenant_id = 't1' AND model = 'gpt-4'`).Scan(&totalInput, &totalOutput, &totalCost, &reqCount)

	if totalInput != 300 {
		t.Errorf("total_input_tokens = %d, want %d", totalInput, 300)
	}
	if totalOutput != 150 {
		t.Errorf("total_output_tokens = %d, want %d", totalOutput, 150)
	}
	if reqCount != 2 {
		t.Errorf("request_count = %d, want %d", reqCount, 2)
	}
	_ = totalCost

	pool.Exec(ctx, `DELETE FROM usage_raw WHERE tenant_id = 't1'`)
	pool.Exec(ctx, `DELETE FROM usage_agg_hourly WHERE tenant_id = 't1'`)
}

// @sk-test 131-analytics-pipeline#T4.3: TestAggregationWorkerDaily (AC-004)
func TestAggregationWorkerDaily(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	now := time.Now().UTC()
	dayBoundary := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	pool.Exec(ctx, `INSERT INTO usage_raw (id, tenant_id, model, input_tokens, output_tokens, cost, recorded_at) VALUES
		(gen_random_uuid(), 't1', 'gpt-4', 500, 250, 0.075, $1),
		(gen_random_uuid(), 't1', 'gpt-4', 300, 150, 0.045, $1)`, dayBoundary)

	worker := NewAggregationWorker(pool, time.Hour, zap.NewNop())
	worker.materializeDaily(ctx, dayBoundary)

	var rowCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM usage_agg_daily WHERE tenant_id = 't1' AND model = 'gpt-4'`).Scan(&rowCount)
	if rowCount != 1 {
		t.Fatalf("expected 1 daily agg row, got %d", rowCount)
	}

	var totalInput, totalOutput int64
	pool.QueryRow(ctx, `SELECT total_input_tokens, total_output_tokens FROM usage_agg_daily WHERE tenant_id = 't1' AND model = 'gpt-4'`).Scan(&totalInput, &totalOutput)

	if totalInput != 800 {
		t.Errorf("total_input_tokens = %d, want %d", totalInput, 800)
	}
	if totalOutput != 400 {
		t.Errorf("total_output_tokens = %d, want %d", totalOutput, 400)
	}

	pool.Exec(ctx, `DELETE FROM usage_raw WHERE tenant_id = 't1'`)
	pool.Exec(ctx, `DELETE FROM usage_agg_daily WHERE tenant_id = 't1'`)
}

// @sk-test 131-analytics-pipeline#T4.3: TestAggregationWorkerNilPool (AC-004)
func TestAggregationWorkerNilPool(t *testing.T) {
	worker := NewAggregationWorker(nil, time.Hour, zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	worker.Run(ctx)
}
