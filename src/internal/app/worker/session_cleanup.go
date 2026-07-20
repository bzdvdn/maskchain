package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

// @sk-task sessions#T5.1: Implement CleanupWorker with ticker-based DeleteExpired (AC-007)
//
// CleanupWorker represents a domain entity or configuration.
type CleanupWorker struct {
	usecase  *session.SessionUseCase
	interval time.Duration
	log      *slog.Logger
}

// @sk-task sessions#T5.1: NewCleanupWorker creates a new worker (AC-007)
//
// NewCleanupWorker creates a new CleanupWorker.
func NewCleanupWorker(usecase *session.SessionUseCase, interval time.Duration, log *slog.Logger) *CleanupWorker {
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

	w.log.Info("session cleanup worker started", slog.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("session cleanup worker stopped", slog.String("error", ctx.Err().Error()))
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
		w.log.Warn("session cleanup: delete expired failed", slog.String("error", err.Error()))
		return
	}
	if deleted > 0 {
		w.log.Info("session cleanup: expired sessions deleted", slog.Int64("count", deleted))
	}
}
