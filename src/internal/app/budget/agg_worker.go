package budgetapp

import (
	"context"
	"log/slog"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
)

// @sk-task 301-budget-enforcement#T2.3: AggregationWorker materializes daily budget spend (AC-005)
//
// AggregationWorker periodically upserts aggregated spend into
// budget_spend_daily for dashboard/history queries.
type AggregationWorker struct {
	repo     budget.BudgetRepository
	interval time.Duration
	log      *slog.Logger
}

func NewAggregationWorker(repo budget.BudgetRepository, interval time.Duration, log *slog.Logger) *AggregationWorker {
	return &AggregationWorker{repo: repo, interval: interval, log: log}
}

// @sk-task 301-budget-enforcement#T2.3: Run starts the aggregation loop (AC-005)
func (w *AggregationWorker) Run(ctx context.Context) {
	if w.repo == nil {
		w.log.Warn("budget aggregation worker: no repository, disabled")
		return
	}
	w.log.Info("budget aggregation worker started", slog.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("budget aggregation worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *AggregationWorker) runOnce(ctx context.Context) {
	now := time.Now().UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	aggCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := w.repo.AggregateDaily(aggCtx, day); err != nil {
		w.log.Warn("budget aggregation worker: daily materialization failed", slog.String("error", err.Error()))
	}
}
