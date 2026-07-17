package session

import (
	"context"
	"time"
)

type ListResult struct {
	Items []Session
	Total int
	Page  int
	Limit int
}

// @sk-task sessions#T1.1: Define SessionStore port interface (AC-001, AC-002, AC-004, AC-005, AC-006, AC-007)
type SessionStore interface {
	Save(ctx context.Context, s *Session) error
	Get(ctx context.Context, tenantID, sessionID string) (*Session, error)
	IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error
	ExtendTTL(ctx context.Context, tenantID, sessionID string, newExpiresAt time.Time) error
	Close(ctx context.Context, tenantID, sessionID string) error
	DeleteExpired(ctx context.Context) (int64, error)
	ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*ListResult, error)
	ListAll(ctx context.Context, page, limit int32) (*ListResult, error)
}
