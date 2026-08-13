package conversation

import (
	"encoding/json"
	"fmt"
	"time"
)

// ConversationStatus is the derived outcome of the shield/upstream processing
// for a captured request.
type ConversationStatus string

const (
	StatusOK      ConversationStatus = "ok"
	StatusBlocked ConversationStatus = "blocked"
	StatusError   ConversationStatus = "error"
)

// @sk-task conversation-logging#T1.3: Implement MaskingEntry entity (RQ-007, AC-001, AC-005)
//
// MaskingEntry is a single "placeholder -> original" mapping proving how the
// shield masked a value for the upstream provider.
type MaskingEntry struct {
	Placeholder string `json:"placeholder"`
	Original    string `json:"original"`
}

// @sk-task conversation-logging#T1.3: Implement NewMaskingEntry factory (RQ-007)
func NewMaskingEntry(placeholder, original string) (MaskingEntry, error) {
	if placeholder == "" {
		return MaskingEntry{}, fmt.Errorf("placeholder must not be empty")
	}
	if original == "" {
		return MaskingEntry{}, fmt.Errorf("original must not be empty")
	}
	return MaskingEntry{Placeholder: placeholder, Original: original}, nil
}

// MaskingEntriesJSON serializes masking entries to the JSON payload that is
// then encrypted before storage (RQ-007).
func MaskingEntriesJSON(entries []MaskingEntry) ([]byte, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	return json.Marshal(entries)
}

// @sk-task conversation-logging#T2.1: ParseMaskingEntriesJSON restores entries from decrypted JSON (RQ-007)
func ParseMaskingEntriesJSON(data []byte) ([]MaskingEntry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var entries []MaskingEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// @sk-task conversation-logging#T2.1: Change Masking to carry ciphertext bytes (AC-004, DEC-004)
// @sk-task conversation-logging#T2.4: Add Streamed flag for streaming captures (RQ-009, DEC-007, AC-003)
// @sk-task conversation-logging#T5.1: Add MaskID linking the log to the shield mask (AC-005)
//
// ConversationLog is a domain entity or configuration.
type ConversationLog struct {
	ID          string
	TenantID    string
	Model       string
	Status      ConversationStatus
	Masked      bool
	Streamed    bool
	MaskID      string
	Detector    string
	Category    string
	Request     []byte
	Response    []byte
	Masking     []byte
	RequestLen  int
	ResponseLen int
	CreatedAt   time.Time
}

// @sk-task conversation-logging#T1.3: NewConversationLog creates a ConversationLog (AC-001)
func NewConversationLog(id, tenantID, model string, status ConversationStatus, request []byte, createdAt time.Time) (*ConversationLog, error) {
	if id == "" {
		return nil, fmt.Errorf("id must not be empty")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID must not be empty")
	}
	if model == "" {
		return nil, fmt.Errorf("model must not be empty")
	}
	switch status {
	case StatusOK, StatusBlocked, StatusError:
	default:
		return nil, fmt.Errorf("invalid status %q", status)
	}
	if len(request) == 0 {
		return nil, fmt.Errorf("request must not be empty")
	}
	if createdAt.IsZero() {
		return nil, fmt.Errorf("createdAt must not be zero")
	}
	return &ConversationLog{
		ID:         id,
		TenantID:   tenantID,
		Model:      model,
		Status:     status,
		Request:    request,
		RequestLen: len(request),
		CreatedAt:  createdAt,
	}, nil
}

// NewMetadataOnlyConversationLog creates a ConversationLog that carries no
// content payloads (request/response/masking empty). It is used for retention
// mode `meta` where only metadata + block facts are persisted (DEC-003, AC-002).
func NewMetadataOnlyConversationLog(id, tenantID, model string, status ConversationStatus, createdAt time.Time) (*ConversationLog, error) {
	if id == "" {
		return nil, fmt.Errorf("id must not be empty")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID must not be empty")
	}
	if model == "" {
		return nil, fmt.Errorf("model must not be empty")
	}
	switch status {
	case StatusOK, StatusBlocked, StatusError:
	default:
		return nil, fmt.Errorf("invalid status %q", status)
	}
	if createdAt.IsZero() {
		return nil, fmt.Errorf("createdAt must not be zero")
	}
	return &ConversationLog{
		ID:         id,
		TenantID:   tenantID,
		Model:      model,
		Status:     status,
		RequestLen: 0,
		CreatedAt:  createdAt,
	}, nil
}

// @sk-task conversation-logging#T1.3: WithResponse sets the response payload and status (AC-001, AC-002)
func (l *ConversationLog) WithResponse(response []byte) *ConversationLog {
	l.Response = response
	l.ResponseLen = len(response)
	return l
}

// @sk-task conversation-logging#T2.1: WithMasking sets the encrypted mask-proof and Masked flag (RQ-007, AC-004)
//
// masking holds the ciphertext of the serialized masking entries.
func (l *ConversationLog) WithMasking(masking []byte) *ConversationLog {
	l.Masking = masking
	l.Masked = len(masking) > 0
	return l
}

// @sk-task conversation-logging#T5.1: WithMaskID sets the shield dict mask id (AC-005)
//
// maskID links this log to the shield dict masking (X-Shield-Dict-Mask-ID).
func (l *ConversationLog) WithMaskID(maskID string) *ConversationLog {
	l.MaskID = maskID
	return l
}
