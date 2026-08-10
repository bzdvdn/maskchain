package conversation

import (
	"context"
	"log/slog"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
)

// @sk-task conversation-logging#T2.1: Implement CleanupWorker with ticker-based TTL cleanup (RQ-008, AC-007)
//
// CleanupWorker periodically removes conversation logs older than the
// configured retention window.
type CleanupWorker struct {
	store     conversation.ConversationStore
	interval  time.Duration
	retention time.Duration
	log       *slog.Logger
}

// @sk-task conversation-logging#T2.1: NewCleanupWorker creates a new cleanup worker (RQ-008, AC-007)
func NewCleanupWorker(store conversation.ConversationStore, interval time.Duration, retention time.Duration, log *slog.Logger) *CleanupWorker {
	return &CleanupWorker{
		store:     store,
		interval:  interval,
		retention: retention,
		log:       log,
	}
}

// @sk-task conversation-logging#T2.1: Run starts the cleanup loop (RQ-008, AC-007)
func (w *CleanupWorker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.log.Warn("conversation cleanup worker: interval <= 0, disabled")
		return
	}

	w.log.Info("conversation cleanup worker started",
		slog.Duration("interval", w.interval),
		slog.Duration("retention", w.retention),
	)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("conversation cleanup worker stopped")
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
		w.log.Warn("conversation cleanup worker: delete failed", slog.String("error", err.Error()))
		return
	}
	if deleted > 0 {
		w.log.Info("conversation cleanup worker: old records deleted", slog.Int64("count", deleted))
	}
}
