package worker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

// @sk-task sessions#T5.1: Implement CleanupWorker with ticker-based DeleteExpired (AC-007)
type CleanupWorker struct {
	usecase  *session.SessionUseCase
	interval time.Duration
	log      *zap.Logger
}

// @sk-task sessions#T5.1: NewCleanupWorker creates a new worker (AC-007)
func NewCleanupWorker(usecase *session.SessionUseCase, interval time.Duration, log *zap.Logger) *CleanupWorker {
	return &CleanupWorker{
		usecase:  usecase,
		interval: interval,
		log:      log,
	}
}

// @sk-task sessions#T5.1: Run starts the cleanup loop with context-based graceful shutdown (AC-007)
func (w *CleanupWorker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.log.Warn("session cleanup worker: interval <= 0, disabled")
		return
	}

	w.log.Info("session cleanup worker started", zap.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("session cleanup worker stopped", zap.Error(ctx.Err()))
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// @sk-task sessions#T5.1: runOnce calls DeleteExpired and logs result (AC-007)
func (w *CleanupWorker) runOnce(ctx context.Context) {
	deleted, err := w.usecase.DeleteExpired(ctx)
	if err != nil {
		w.log.Warn("session cleanup: delete expired failed", zap.Error(err))
		return
	}
	if deleted > 0 {
		w.log.Info("session cleanup: expired sessions deleted", zap.Int64("count", deleted))
	}
}
