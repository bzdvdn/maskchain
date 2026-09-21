package logexport

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

type recordingSink struct {
	name    string
	mu      sync.Mutex
	batches [][]domainlogexport.Record
	err     error
}

func (s *recordingSink) Name() string { return s.name }

func (s *recordingSink) Send(_ context.Context, records []domainlogexport.Record) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	batch := make([]domainlogexport.Record, len(records))
	copy(batch, records)
	s.batches = append(s.batches, batch)
	return nil
}

func (s *recordingSink) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, b := range s.batches {
		n += len(b)
	}
	return n
}

func testRecord(tenant string) domainlogexport.Record {
	return domainlogexport.Record{Tenant: tenant, Model: "m", Status: "OK"}
}

// @sk-test log-export#T4.2: records fan out only to their tenant's sinks (AC-003)
func TestPipelineFanOutByTenant(t *testing.T) {
	metrics.Reset()
	sinkA := &recordingSink{name: "a"}
	sinkB := &recordingSink{name: "b"}
	resolver := domainlogexport.NewResolver(map[string][]string{"alpha": {"a"}, "beta": {"b"}})
	p := NewPipeline([]domainlogexport.Sink{sinkA, sinkB}, resolver, 10, 10, 0, nil)

	p.deliver(context.Background(), []domainlogexport.Record{testRecord("alpha"), testRecord("beta")})

	if sinkA.total() != 1 || sinkB.total() != 1 {
		t.Fatalf("expected 1 record per sink, got a=%d b=%d", sinkA.total(), sinkB.total())
	}
	if sinkA.batches[0][0].Tenant != "alpha" || sinkB.batches[0][0].Tenant != "beta" {
		t.Error("records were routed to the wrong tenant's sink")
	}
}

// @sk-test log-export#T4.2: a failing sink is isolated and counted (AC-008)
func TestPipelineFailureIsolation(t *testing.T) {
	metrics.Reset()
	bad := &recordingSink{name: "bad", err: errors.New("boom")}
	good := &recordingSink{name: "good"}
	resolver := domainlogexport.NewResolver(map[string][]string{"alpha": {"bad", "good"}})
	p := NewPipeline([]domainlogexport.Sink{bad, good}, resolver, 10, 10, 0, nil)

	p.deliver(context.Background(), []domainlogexport.Record{testRecord("alpha")})

	if good.total() != 1 {
		t.Errorf("healthy sink should still receive the record, got %d", good.total())
	}
	if bad.total() != 0 {
		t.Errorf("failing sink should receive nothing, got %d", bad.total())
	}
	if got := testutil.ToFloat64(metrics.LogExportFailedTotal.WithLabelValues("bad")); got != 1 {
		t.Errorf("failed metric = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.LogExportDeliveredTotal.WithLabelValues("good")); got != 1 {
		t.Errorf("delivered metric = %v, want 1", got)
	}
}

// @sk-test log-export#T4.2: an unrouted tenant is never delivered (AC-001)
func TestPipelineUnroutedTenantDrops(t *testing.T) {
	metrics.Reset()
	sink := &recordingSink{name: "a"}
	p := NewPipeline([]domainlogexport.Sink{sink}, domainlogexport.NewResolver(nil), 10, 10, 0, nil)

	p.deliver(context.Background(), []domainlogexport.Record{testRecord("alpha")})

	if sink.total() != 0 {
		t.Errorf("unrouted tenant must not be delivered, got %d", sink.total())
	}
}

// @sk-test log-export#T4.2: a full queue drops with a metric (AC-008)
func TestPipelineQueueOverflowDrops(t *testing.T) {
	metrics.Reset()
	p := NewPipeline(nil, domainlogexport.NewResolver(nil), 10, 1, 0, nil)

	p.Enqueue(testRecord("alpha"))
	p.Enqueue(testRecord("alpha")) // queue is full

	if got := testutil.ToFloat64(metrics.LogExportDroppedTotal.WithLabelValues("queue_full")); got != 1 {
		t.Errorf("dropped metric = %v, want 1", got)
	}
}
