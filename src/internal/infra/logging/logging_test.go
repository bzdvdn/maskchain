package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// @sk-test 61-observability#T4.1: TestOTelHandler_TraceID verifies slog entry contains trace_id and span_id (AC-005)
func TestOTelHandler_TraceID(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, slog.LevelInfo)

	spanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.TraceFlags(1),
		Remote:     false,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	log.InfoContext(ctx, "test message", "key", "value")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["trace_id"] != traceID.String() {
		t.Errorf("expected trace_id=%s, got %v", traceID.String(), entry["trace_id"])
	}
	if entry["span_id"] != spanID.String() {
		t.Errorf("expected span_id=%s, got %v", spanID.String(), entry["span_id"])
	}
	if entry["msg"] != "test message" {
		t.Errorf("expected msg=test message, got %v", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key=value, got %v", entry["key"])
	}
}

// @sk-test 61-observability#T4.1: TestOTelHandler_NoSpan verifies no trace fields when no span in context (AC-005)
func TestOTelHandler_NoSpan(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, slog.LevelInfo)

	log.InfoContext(context.Background(), "no span test")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if _, ok := entry["trace_id"]; ok {
		t.Error("expected no trace_id when no span in context")
	}
	if _, ok := entry["span_id"]; ok {
		t.Error("expected no span_id when no span in context")
	}
}
