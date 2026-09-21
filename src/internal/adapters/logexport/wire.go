package logexport

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
)

// wireRecord is the JSON shape exported to sinks. It carries masked content and
// metadata only.
type wireRecord struct {
	EventID   string       `json:"event_id"`
	Timestamp string       `json:"timestamp"`
	Tenant    string       `json:"tenant"`
	Model     string       `json:"model"`
	Status    string       `json:"status"`
	Tokens    int64        `json:"tokens"`
	Cost      float64      `json:"cost"`
	MaskID    string       `json:"mask_id,omitempty"`
	Content   *wireContent `json:"content,omitempty"`
}

type wireContent struct {
	MaskedRequest  string `json:"masked_request"`
	MaskedResponse string `json:"masked_response"`
}

func toWire(records []domainlogexport.Record) []wireRecord {
	out := make([]wireRecord, 0, len(records))
	for _, r := range records {
		wr := wireRecord{
			EventID:   r.EventID,
			Timestamp: r.Timestamp.UTC().Format(time.RFC3339Nano),
			Tenant:    r.Tenant,
			Model:     r.Model,
			Status:    r.Status,
			Tokens:    r.Tokens,
			Cost:      r.Cost,
			MaskID:    r.MaskID,
		}
		if r.Content != nil {
			wr.Content = &wireContent{
				MaskedRequest:  r.Content.MaskedRequest,
				MaskedResponse: r.Content.MaskedResponse,
			}
		}
		out = append(out, wr)
	}
	return out
}

func newEventID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
