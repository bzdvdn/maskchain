package logging

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// @sk-task 61-observability#T1.4: NewLogger creates a slog.Logger with JSON handler (AC-005)
//
// NewLogger creates a new Logger.
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(newOTelHandler(w, &slog.HandlerOptions{Level: level}))
}

// @sk-task 61-observability#T1.4: newOTelHandler creates a handler that enriches logs with trace_id/span_id (AC-005)
func newOTelHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &otelHandler{
		inner: slog.NewJSONHandler(w, opts),
	}
}

type otelHandler struct {
	inner slog.Handler
}

func (h *otelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *otelHandler) Handle(ctx context.Context, record slog.Record) error {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() && spanCtx.IsSampled() {
		record.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, record)
}

func (h *otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &otelHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *otelHandler) WithGroup(name string) slog.Handler {
	return &otelHandler{inner: h.inner.WithGroup(name)}
}
