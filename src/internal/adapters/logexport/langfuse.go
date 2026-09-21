package logexport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
)

// @sk-task log-export#T3.2: Langfuse ingestion sink (AC-007)
//
// LangfuseSink submits masked input/output plus metadata to the Langfuse
// ingestion API as a batch of trace events.
type LangfuseSink struct {
	name      string
	host      string
	publicKey string
	secretKey string
	client    *http.Client
}

// NewLangfuseSink builds a Langfuse sink.
func NewLangfuseSink(name, host, publicKey, secretKey string, timeout time.Duration) *LangfuseSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &LangfuseSink{
		name:      name,
		host:      strings.TrimRight(host, "/"),
		publicKey: publicKey,
		secretKey: secretKey,
		client:    &http.Client{Timeout: timeout},
	}
}

func (s *LangfuseSink) Name() string { return s.name }

type langfuseEvent struct {
	Type      string         `json:"type"`
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Body      map[string]any `json:"body"`
}

// Send posts a batch of trace-create events to the Langfuse ingestion API.
func (s *LangfuseSink) Send(ctx context.Context, records []domainlogexport.Record) error {
	if len(records) == 0 {
		return nil
	}
	batch := make([]langfuseEvent, 0, len(records))
	for _, r := range records {
		ts := r.Timestamp.UTC().Format(time.RFC3339Nano)
		body := map[string]any{
			"id":        r.EventID,
			"timestamp": ts,
			"name":      "maskchain",
			"metadata": map[string]any{
				"tenant":  r.Tenant,
				"model":   r.Model,
				"status":  r.Status,
				"tokens":  r.Tokens,
				"cost":    r.Cost,
				"mask_id": r.MaskID,
			},
		}
		if r.Content != nil {
			body["input"] = r.Content.MaskedRequest
			body["output"] = r.Content.MaskedResponse
		}
		batch = append(batch, langfuseEvent{Type: "trace-create", ID: r.EventID, Timestamp: ts, Body: body})
	}

	payload, err := json.Marshal(map[string]any{"batch": batch})
	if err != nil {
		return fmt.Errorf("langfuse sink %s: encode: %w", s.name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.host+"/api/public/ingestion", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("langfuse sink %s: request: %w", s.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(s.publicKey, s.secretKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("langfuse sink %s: %w", s.name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("langfuse sink %s: status %d", s.name, resp.StatusCode)
	}
	return nil
}
