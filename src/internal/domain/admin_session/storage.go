package admin_session

import (
	"context"
	"time"
)

// @sk-task admin-ui-design#T1.3: AdminSessionStore port interface (AC-001, AC-004)
// @sk-task admin-session-sliding#T1.2: UpdateExpiry for sliding renewal (AC-001)
//
// AdminSessionStore defines the interface for domain operations.
type AdminSessionStore interface {
	Save(ctx context.Context, s *AdminSession) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*AdminSession, error)
	UpdateExpiry(ctx context.Context, sessionID string, expiresAt time.Time) error
	Delete(ctx context.Context, sessionID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}
