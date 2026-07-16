package analytics

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
)

// @sk-task 131-analytics-pipeline#T3.5: Implement CleanupWorker with ticker-based retention cleanup (AC-008)
type CleanupWorker struct {
	store     analytics.UsageStore
	interval  time.Duration
	retention time.Duration
	log       *zap.Logger
}

// @sk-task 131-analytics-pipeline#T3.5: NewCleanupWorker creates a new cleanup worker (AC-008)
func NewCleanupWorker(store analytics.UsageStore, interval time.Duration, retention time.Duration, log *zap.Logger) *CleanupWorker {
	return &CleanupWorker{
		store:     store,
		interval:  interval,
		retention: retention,
		log:       log,
	}
}

// @sk-task 131-analytics-pipeline#T3.5: Run starts the cleanup loop (AC-008)
func (w *CleanupWorker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.log.Warn("cleanup worker: interval <= 0, disabled")
		return
	}

	w.log.Info("cleanup worker started",
		zap.Duration("interval", w.interval),
		zap.Duration("retention", w.retention),
	)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("cleanup worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *CleanupWorker) runOnce(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-w.retention)
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	deleted, err := w.store.DeleteOlderThan(cleanupCtx, cutoff)
	if err != nil {
		w.log.Warn("cleanup worker: delete failed", zap.Error(err))
		return
	}
	if deleted > 0 {
		w.log.Info("cleanup worker: old records deleted", zap.Int64("count", deleted))
	}
}
