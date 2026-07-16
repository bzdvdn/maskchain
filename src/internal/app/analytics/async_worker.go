package analytics

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
)

// @sk-task 131-analytics-pipeline#T3.2: Implement AsyncWorker with buffered channel and ticker-based batch insert (AC-002)
type AsyncWorker struct {
	store    analytics.UsageStore
	buffer   chan analytics.TokenUsage
	interval time.Duration
	log      *zap.Logger
}

// @sk-task 131-analytics-pipeline#T3.2: NewAsyncWorker creates a new async worker (AC-002)
func NewAsyncWorker(store analytics.UsageStore, bufferSize int, interval time.Duration, log *zap.Logger) *AsyncWorker {
	return &AsyncWorker{
		store:    store,
		buffer:   make(chan analytics.TokenUsage, bufferSize),
		interval: interval,
		log:      log,
	}
}

// @sk-task 131-analytics-pipeline#T3.2: Run starts the worker loop with ticker-based flush and context shutdown (AC-002)
func (w *AsyncWorker) Run(ctx context.Context) {
	w.log.Info("async worker started", zap.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	var batch []analytics.TokenUsage

	flush := func() {
		if len(batch) == 0 {
			return
		}
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := w.store.RecordBatch(flushCtx, batch); err != nil {
			w.log.Warn("async worker: batch insert failed", zap.Error(err), zap.Int("size", len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			w.log.Info("async worker stopped")
			return
		case <-ticker.C:
			flush()
		case usage := <-w.buffer:
			batch = append(batch, usage)
			if len(batch) >= cap(w.buffer) {
				flush()
			}
		}
	}
}

// @sk-task 131-analytics-pipeline#T3.2: Send sends a TokenUsage to the buffer channel (non-blocking) (AC-002)
func (w *AsyncWorker) Send(usage analytics.TokenUsage) {
	select {
	case w.buffer <- usage:
	default:
		w.log.Warn("async worker: buffer full, dropping record")
	}
}

// @sk-task 131-analytics-pipeline#T3.2: Buffer returns the send-only channel for direct use (AC-002)
func (w *AsyncWorker) Buffer() chan<- analytics.TokenUsage {
	return w.buffer
}
