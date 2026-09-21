package logexport

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
)

// SignatureHeader carries the HMAC-SHA256 signature of the webhook body.
const SignatureHeader = "X-MaskChain-Signature"

// @sk-task log-export#T2.3: signed webhook sink (AC-005)
//
// WebhookSink delivers a JSON batch signed with HMAC-SHA256 over the exact body,
// plus an event id and timestamp so the receiver can verify and deduplicate.
type WebhookSink struct {
	name   string
	url    string
	secret string
	client *http.Client
}

// NewWebhookSink builds a webhook sink.
func NewWebhookSink(name, url, secret string, timeout time.Duration) *WebhookSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &WebhookSink{
		name:   name,
		url:    url,
		secret: secret,
		client: &http.Client{Timeout: timeout},
	}
}

func (s *WebhookSink) Name() string { return s.name }

type webhookEnvelope struct {
	EventID   string       `json:"event_id"`
	Timestamp string       `json:"timestamp"`
	Records   []wireRecord `json:"records"`
}

// Send signs and delivers a batch. Any non-2xx response is an error.
func (s *WebhookSink) Send(ctx context.Context, records []domainlogexport.Record) error {
	eventID := newEventID()
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	body, err := json.Marshal(webhookEnvelope{EventID: eventID, Timestamp: ts, Records: toWire(records)})
	if err != nil {
		return fmt.Errorf("webhook sink %s: encode: %w", s.name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook sink %s: request: %w", s.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, SignPayload(s.secret, body))
	req.Header.Set("X-MaskChain-Event", eventID)
	req.Header.Set("X-MaskChain-Timestamp", ts)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook sink %s: %w", s.name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook sink %s: status %d", s.name, resp.StatusCode)
	}
	return nil
}

// SignPayload returns the signature header value for a body and secret. It is
// exported so receivers and tests can verify deliveries.
func SignPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
