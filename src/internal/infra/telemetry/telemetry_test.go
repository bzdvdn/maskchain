package telemetry

import (
	"context"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	coll "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metriccoll "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// @sk-test 61-observability#T4.1: TestInitProvider_EmptyEndpoint returns noop shutdown (AC-007)
func TestInitProvider_EmptyEndpoint(t *testing.T) {
	log := zap.NewNop()
	shutdown, err := InitProvider(context.Background(), "", "test-service", "test", 1.0, log)
	if err != nil {
		t.Fatalf("InitProvider failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown should not error: %v", err)
	}
}

// @sk-test 61-observability#T4.1: TestInitProvider_Shutdown verifies shutdown func flushes TracerProvider (AC-006)
func TestInitProvider_Shutdown(t *testing.T) {
	_, srv, addr := startMockOTLPServer(t)
	defer srv.Stop()

	log := zap.NewNop()
	shutdown, err := InitProvider(context.Background(), addr, "test-service", "test", 1.0, log)
	if err != nil {
		t.Fatalf("InitProvider failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}

	tracer := otel.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown should not error: %v", err)
	}
}

// @sk-test 61-observability#T4.1: TestInitProvider_UnreachableEndpoint noops gracefully when endpoint unreachable (AC-007)
func TestInitProvider_UnreachableEndpoint(t *testing.T) {
	log := zap.NewNop()
	shutdown, err := InitProvider(context.Background(), "localhost:1", "test-service", "test", 1.0, log)
	if err != nil {
		t.Fatalf("InitProvider should not error on unreachable endpoint: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = shutdown(ctx)
}

// @sk-test 61-observability#T4.1: TestInitProvider_WithMockExporter verifies span export via mock OTLP receiver (AC-001)
func TestInitProvider_WithMockExporter(t *testing.T) {
	mock, srv, addr := startMockOTLPServer(t)
	defer srv.Stop()

	log := zap.NewNop()
	shutdown, err := InitProvider(context.Background(), addr, "test-service", "test", 1.0, log)
	if err != nil {
		t.Fatalf("InitProvider failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}

	tracer := otel.Tracer("test")
	_, span := tracer.Start(context.Background(), "/health",
		trace.WithAttributes(attribute.String("http.method", "GET")),
	)
	span.SetAttributes(attribute.Int("http.status_code", 200))
	span.End()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if len(mock.spans) == 0 {
		t.Fatal("expected at least one exported span")
	}

	found := false
	for _, s := range mock.spans {
		if s.Name == "/health" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected span named /health, got spans: %v", spanNames(mock.spans))
	}
}

// @sk-test 61-observability#T4.1: TestInitProvider_EndpointConfig verifies OTLP endpoint config is accepted (AC-001)
func TestInitProvider_EndpointConfig(t *testing.T) {
	_, srv, addr := startMockOTLPServer(t)
	defer srv.Stop()

	log := zap.NewNop()
	shutdown, err := InitProvider(context.Background(), addr, "test-service", "test", 1.0, log)
	if err != nil {
		t.Fatalf("InitProvider should accept endpoint config: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = shutdown(ctx)
}

type mockTraceServer struct {
	coll.UnimplementedTraceServiceServer
	spans []*tracepb.Span
}

func (m *mockTraceServer) Export(ctx context.Context, req *coll.ExportTraceServiceRequest) (*coll.ExportTraceServiceResponse, error) {
	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			m.spans = append(m.spans, ss.Spans...)
		}
	}
	return &coll.ExportTraceServiceResponse{}, nil
}

type mockMetricsServer struct {
	metriccoll.UnimplementedMetricsServiceServer
}

func (m *mockMetricsServer) Export(ctx context.Context, req *metriccoll.ExportMetricsServiceRequest) (*metriccoll.ExportMetricsServiceResponse, error) {
	return &metriccoll.ExportMetricsServiceResponse{}, nil
}

func startMockOTLPServer(t *testing.T) (*mockTraceServer, *grpc.Server, string) {
	t.Helper()
	mock := new(mockTraceServer)
	mockM := new(mockMetricsServer)
	srv, addr := startGRPCServer(t, mock, mockM)
	return mock, srv, addr
}

func startGRPCServer(t *testing.T, traceSvc coll.TraceServiceServer, metricSvc metriccoll.MetricsServiceServer) (*grpc.Server, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	srv := grpc.NewServer()
	coll.RegisterTraceServiceServer(srv, traceSvc)
	metriccoll.RegisterMetricsServiceServer(srv, metricSvc)
	go srv.Serve(lis)
	return srv, lis.Addr().String()
}

func spanNames(spans []*tracepb.Span) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name
	}
	return names
}

func init() {
	// Ensure global tracer provider is reset before any test that uses otel.Tracer
	otel.SetTracerProvider(sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.Default()),
		sdktrace.WithSampler(sdktrace.NeverSample()),
	))
}
