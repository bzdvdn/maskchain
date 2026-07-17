package admin_session

import "context"

// @sk-task admin-ui-design#T1.3: AdminSessionStore port interface (AC-001, AC-004)
type AdminSessionStore interface {
	Save(ctx context.Context, s *AdminSession) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*AdminSession, error)
	Delete(ctx context.Context, sessionID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}
