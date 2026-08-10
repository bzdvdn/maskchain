package conversation

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by Get when a conversation record does not exist.
var ErrNotFound = errors.New("conversation: not found")

// ConversationFilter narrows List results by tenant, status, model and pagination.
type ConversationFilter struct {
	TenantID string
	Status   ConversationStatus
	Model    string
	Page     int
	PerPage  int
}

// ConversationPage is a paginated metadata-only list, ordered by CreatedAt.
type ConversationPage struct {
	Items []ConversationLog
	Total int
}

// @sk-task conversation-logging#T1.3: Implement ConversationStore port (AC-001, AC-003, AC-005, AC-006, AC-007)
//
// ConversationStore defines the interface for domain operations.
type ConversationStore interface {
	// SaveBatch persists encrypted conversation logs idempotently (ON CONFLICT DO NOTHING).
	SaveBatch(ctx context.Context, logs []ConversationLog) error
	// DeleteOlderThan removes records older than before for TTL cleanup.
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
	// List returns metadata-only records (no payload content) with pagination and tenant filter.
	List(ctx context.Context, filter ConversationFilter) (ConversationPage, error)
	// Get returns a single record including encrypted content for detail decoding.
	Get(ctx context.Context, id string) (*ConversationLog, error)
}
