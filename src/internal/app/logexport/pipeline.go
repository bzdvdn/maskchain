package logexport

import (
	"context"
	"log/slog"
	"time"

	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

const (
	deliveryAttempts = 3
	retryBackoff     = 20 * time.Millisecond
	flushInterval    = 250 * time.Millisecond
)

// @sk-task log-export#T2.1: bounded async export pipeline (AC-001, AC-003, AC-008)
//
// Pipeline batches captured records and fans them out to the sinks enabled for
// each record's tenant. Delivery never blocks the request path: the queue is
// bounded and drops with a metric on overflow, and a failing sink never blocks
// delivery to the other sinks.
type Pipeline struct {
	sinks     map[string]domainlogexport.Sink
	resolver  *domainlogexport.Resolver
	batchSize int
	timeout   time.Duration
	queue     chan domainlogexport.Record
	log       *slog.Logger
}

// NewPipeline builds the pipeline. Non-positive sizes fall back to safe defaults.
func NewPipeline(sinks []domainlogexport.Sink, resolver *domainlogexport.Resolver, batchSize, queueSize int, timeout time.Duration, log *slog.Logger) *Pipeline {
	m := make(map[string]domainlogexport.Sink, len(sinks))
	for _, s := range sinks {
		if s != nil {
			m[s.Name()] = s
		}
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	if queueSize <= 0 {
		queueSize = 1024
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if log == nil {
		log = slog.Default()
	}
	return &Pipeline{
		sinks:     m,
		resolver:  resolver,
		batchSize: batchSize,
		timeout:   timeout,
		queue:     make(chan domainlogexport.Record, queueSize),
		log:       log,
	}
}

// Enqueue records a record for delivery. It never blocks the caller.
func (p *Pipeline) Enqueue(rec domainlogexport.Record) {
	select {
	case p.queue <- rec:
	default:
		metrics.LogExportDroppedTotal.WithLabelValues("queue_full").Inc()
		p.log.Warn("log export: queue full, dropping record", slog.String("tenant", rec.Tenant))
	}
}

// Run consumes the queue until ctx is cancelled, flushing in batches.
func (p *Pipeline) Run(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]domainlogexport.Record, 0, p.batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		p.deliver(ctx, batch)
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case rec := <-p.queue:
			batch = append(batch, rec)
			if len(batch) >= p.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// deliver groups records by their tenant's sinks and sends each group.
func (p *Pipeline) deliver(ctx context.Context, records []domainlogexport.Record) {
	if len(records) == 0 {
		return
	}
	bySink := make(map[string][]domainlogexport.Record)
	for _, rec := range records {
		for _, name := range p.resolver.SinksFor(rec.Tenant) {
			if _, ok := p.sinks[name]; !ok {
				continue
			}
			bySink[name] = append(bySink[name], rec)
		}
	}

	for name, group := range bySink {
		if err := p.send(ctx, p.sinks[name], group); err != nil {
			metrics.LogExportFailedTotal.WithLabelValues(name).Inc()
			p.log.Warn("log export: delivery failed",
				slog.String("sink", name), slog.Int("records", len(group)), slog.String("error", err.Error()))
			continue
		}
		metrics.LogExportDeliveredTotal.WithLabelValues(name).Add(float64(len(group)))
	}
}

// send attempts delivery with bounded retries and a per-attempt timeout.
func (p *Pipeline) send(ctx context.Context, sink domainlogexport.Sink, records []domainlogexport.Record) error {
	var err error
	for attempt := 1; attempt <= deliveryAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, p.timeout)
		err = sink.Send(attemptCtx, records)
		cancel()
		if err == nil {
			return nil
		}
		if attempt < deliveryAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryBackoff * time.Duration(attempt)):
			}
		}
	}
	return err
}
