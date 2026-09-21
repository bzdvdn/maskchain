package logexport

import (
	"context"
	"time"
)

// @sk-task log-export#T1.1: masked export record and sink port (AC-002, AC-005)
//
// Content carries the masked request/response text. It is present only when the
// tenant's retention mode allows content export.
type Content struct {
	MaskedRequest  string
	MaskedResponse string
}

// Record is one exportable request/response record. It carries only masked
// content and metadata: original values and the placeholder->original mask
// mapping are never part of a record.
type Record struct {
	EventID   string
	Timestamp time.Time
	Tenant    string
	Model     string
	Status    string
	Tokens    int64
	Cost      float64
	MaskID    string
	// Content is nil for metadata-only exports (retention mode "meta").
	Content *Content
}

// Sink delivers a batch of records to an external system. Implementations must
// be safe for concurrent use and must not mutate the records.
type Sink interface {
	// Name is the sink identifier referenced by per-tenant routing.
	Name() string
	// Send delivers a batch. Returning an error marks the batch as failed for
	// this sink only; the caller retries and never blocks other sinks.
	Send(ctx context.Context, records []Record) error
}
