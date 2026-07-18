package analyticsrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 131-analytics-pipeline#T2.3: Implement PgUsageStore (AC-002, AC-004, AC-008)
type PgUsageStore struct {
	pool *pgxpool.Pool
}

func NewPgUsageStore(pool *pgxpool.Pool) *PgUsageStore {
	return &PgUsageStore{pool: pool}
}

func (s *PgUsageStore) Record(ctx context.Context, usage analytics.TokenUsage) error {
	if s.pool == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO usage_raw (id, tenant_id, model, input_tokens, output_tokens, cost, recorded_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (id) DO NOTHING`,
		uuid.NewString(), usage.TenantID.String(), usage.Model,
		usage.InputTokens, usage.OutputTokens, usage.Cost, usage.Timestamp)
	return err
}

func (s *PgUsageStore) RecordBatch(ctx context.Context, usages []analytics.TokenUsage) error {
	if s.pool == nil || len(usages) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, u := range usages {
		batch.Queue(
			`INSERT INTO usage_raw (id, tenant_id, model, input_tokens, output_tokens, cost, recorded_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (id) DO NOTHING`,
			uuid.NewString(), u.TenantID.String(), u.Model,
			u.InputTokens, u.OutputTokens, u.Cost, u.Timestamp)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range usages {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *PgUsageStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if s.pool == nil {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM usage_raw WHERE recorded_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *PgUsageStore) QueryByTenant(ctx context.Context, tenantID value.TenantID, from, to time.Time) ([]analytics.UsageRecord, error) {
	if s.pool == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT tenant_id, model, recorded_at, recorded_at, input_tokens, output_tokens, cost, 1
		 FROM usage_raw WHERE tenant_id = $1 AND recorded_at >= $2 AND recorded_at <= $3
		 ORDER BY recorded_at`, tenantID.String(), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []analytics.UsageRecord
	for rows.Next() {
		var r analytics.UsageRecord
		if err := rows.Scan(&r.TenantID, &r.Model, &r.PeriodStart, &r.PeriodEnd,
			&r.TotalInputTokens, &r.TotalOutputTokens, &r.TotalCost, &r.RequestCount); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *PgUsageStore) QueryAll(ctx context.Context, from, to time.Time) ([]analytics.UsageRecord, error) {
	if s.pool == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT tenant_id, model, recorded_at, recorded_at, input_tokens, output_tokens, cost, 1
		 FROM usage_raw WHERE recorded_at >= $1 AND recorded_at <= $2
		 ORDER BY recorded_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []analytics.UsageRecord
	for rows.Next() {
		var r analytics.UsageRecord
		if err := rows.Scan(&r.TenantID, &r.Model, &r.PeriodStart, &r.PeriodEnd,
			&r.TotalInputTokens, &r.TotalOutputTokens, &r.TotalCost, &r.RequestCount); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *PgUsageStore) QueryByModel(ctx context.Context, model string, from, to time.Time) ([]analytics.UsageRecord, error) {
	if s.pool == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT tenant_id, model, recorded_at, recorded_at, input_tokens, output_tokens, cost, 1
		 FROM usage_raw WHERE model = $1 AND recorded_at >= $2 AND recorded_at <= $3
		 ORDER BY recorded_at`, model, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []analytics.UsageRecord
	for rows.Next() {
		var r analytics.UsageRecord
		if err := rows.Scan(&r.TenantID, &r.Model, &r.PeriodStart, &r.PeriodEnd,
			&r.TotalInputTokens, &r.TotalOutputTokens, &r.TotalCost, &r.RequestCount); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *PgUsageStore) QueryTimeSeries(ctx context.Context, from, to time.Time) ([]analytics.TimeSeriesPoint, error) {
	if s.pool == nil {
		return nil, nil
	}
	bucket := resolveBucket(from, to)
	rows, err := s.pool.Query(ctx,
		fmt.Sprintf(`SELECT date_trunc('%s', recorded_at) AS bucket,
		        SUM(input_tokens) AS input_tokens,
		        SUM(output_tokens) AS output_tokens,
		        SUM(cost) AS cost, COUNT(*) AS requests
		 FROM usage_raw WHERE recorded_at >= $1 AND recorded_at <= $2
		 GROUP BY date_trunc('%s', recorded_at)
		 ORDER BY bucket`, bucket, bucket), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pts []analytics.TimeSeriesPoint
	for rows.Next() {
		var p analytics.TimeSeriesPoint
		if err := rows.Scan(&p.Bucket, &p.InputTokens, &p.OutputTokens, &p.Cost, &p.Requests); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

func resolveBucket(from, to time.Time) string {
	if to.Sub(from) < 48*time.Hour {
		return "hour"
	}
	return "day"
}

func (s *PgUsageStore) AggregateByDay(ctx context.Context, tenantID value.TenantID, from, to time.Time) ([]analytics.Aggregation, error) {
	if s.pool == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT tenant_id, model, date_trunc('day', recorded_at)::date AS day,
		        SUM(input_tokens + output_tokens) AS total_tokens,
		        SUM(cost) AS total_cost, COUNT(*) AS request_count,
		        '0s'::interval AS avg_latency
		 FROM usage_raw WHERE tenant_id = $1 AND recorded_at >= $2 AND recorded_at <= $3
		 GROUP BY tenant_id, model, date_trunc('day', recorded_at)
		 ORDER BY day`, tenantID.String(), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var aggs []analytics.Aggregation
	for rows.Next() {
		var a analytics.Aggregation
		if err := rows.Scan(&a.TenantID, &a.Model, &a.Date,
			&a.TotalTokens, &a.TotalCost, &a.RequestCount, &a.AvgLatency); err != nil {
			return nil, err
		}
		aggs = append(aggs, a)
	}
	return aggs, rows.Err()
}
