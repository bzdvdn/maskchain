package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// @sk-task 131-analytics-pipeline#T3.4: Implement AggregationWorker with ticker-based materialization (AC-004)
type AggregationWorker struct {
	pool     *pgxpool.Pool
	interval time.Duration
	log      *zap.Logger
}

// @sk-task 131-analytics-pipeline#T3.4: NewAggregationWorker creates a new aggregation worker (AC-004)
func NewAggregationWorker(pool *pgxpool.Pool, interval time.Duration, log *zap.Logger) *AggregationWorker {
	return &AggregationWorker{
		pool:     pool,
		interval: interval,
		log:      log,
	}
}

// @sk-task 131-analytics-pipeline#T3.4: Run starts the aggregation loop (AC-004)
func (w *AggregationWorker) Run(ctx context.Context) {
	if w.pool == nil {
		w.log.Warn("aggregation worker: no pool, disabled")
		return
	}

	w.log.Info("aggregation worker started", zap.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("aggregation worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *AggregationWorker) runOnce(ctx context.Context) {
	now := time.Now().UTC()
	hourBoundary := now.Truncate(time.Hour)
	dayBoundary := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	aggCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := w.materializeHourly(aggCtx, hourBoundary); err != nil {
		w.log.Warn("aggregation worker: hourly materialization failed", zap.Error(err))
		return
	}
	if err := w.materializeDaily(aggCtx, dayBoundary); err != nil {
		w.log.Warn("aggregation worker: daily materialization failed", zap.Error(err))
		return
	}
}

func (w *AggregationWorker) materializeHourly(ctx context.Context, hour time.Time) error {
	_, err := w.pool.Exec(ctx, `
		INSERT INTO usage_agg_hourly (tenant_id, model, hour, total_input_tokens, total_output_tokens, total_cost, request_count, updated_at)
		SELECT
			u.tenant_id,
			u.model,
			date_trunc('hour', u.recorded_at) AS hour,
			COALESCE(SUM(u.input_tokens), 0),
			COALESCE(SUM(u.output_tokens), 0),
			COALESCE(SUM(u.cost), 0),
			COUNT(*) AS request_count,
			NOW()
		FROM usage_raw u
		WHERE u.recorded_at >= $1 AND u.recorded_at < $1 + interval '1 hour'
		GROUP BY u.tenant_id, u.model, date_trunc('hour', u.recorded_at)
		ON CONFLICT (tenant_id, model, hour) DO UPDATE SET
			total_input_tokens  = EXCLUDED.total_input_tokens,
			total_output_tokens = EXCLUDED.total_output_tokens,
			total_cost          = EXCLUDED.total_cost,
			request_count       = EXCLUDED.request_count,
			updated_at          = NOW()
	`, hour)
	return err
}

func (w *AggregationWorker) materializeDaily(ctx context.Context, day time.Time) error {
	_, err := w.pool.Exec(ctx, `
		INSERT INTO usage_agg_daily (tenant_id, model, day, total_input_tokens, total_output_tokens, total_cost, request_count, updated_at)
		SELECT
			u.tenant_id,
			u.model,
			u.recorded_at::date AS day,
			COALESCE(SUM(u.input_tokens), 0),
			COALESCE(SUM(u.output_tokens), 0),
			COALESCE(SUM(u.cost), 0),
			COUNT(*) AS request_count,
			NOW()
		FROM usage_raw u
		WHERE u.recorded_at >= $1 AND u.recorded_at < $1 + interval '1 day'
		GROUP BY u.tenant_id, u.model, u.recorded_at::date
		ON CONFLICT (tenant_id, model, day) DO UPDATE SET
			total_input_tokens  = EXCLUDED.total_input_tokens,
			total_output_tokens = EXCLUDED.total_output_tokens,
			total_cost          = EXCLUDED.total_cost,
			request_count       = EXCLUDED.request_count,
			updated_at          = NOW()
	`, day)
	return err
}
