package admin_session

import (
	"context"
	"fmt"
	"time"
)

// @sk-task admin-ui-design#T1.3: AdminSessionUseCase (AC-001, AC-004)
//
// AdminSessionUseCase represents a domain entity or configuration.
type AdminSessionUseCase struct {
	store AdminSessionStore
}

func NewAdminSessionUseCase(store AdminSessionStore) *AdminSessionUseCase {
	return &AdminSessionUseCase{store: store}
}

func (uc *AdminSessionUseCase) Create(ctx context.Context, username string, ttl time.Duration) (*AdminSession, string, error) {
	session, token, err := NewAdminSession(username, ttl)
	if err != nil {
		return nil, "", fmt.Errorf("create admin session: %w", err)
	}
	if err := uc.store.Save(ctx, session); err != nil {
		return nil, "", fmt.Errorf("save admin session: %w", err)
	}
	return session, token, nil
}

func (uc *AdminSessionUseCase) Validate(ctx context.Context, rawToken string) (*AdminSession, error) {
	tokenHash := HashToken(rawToken)
	s, err := uc.store.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("validate admin session: %w", err)
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, ErrSessionExpired
	}
	return s, nil
}

func (uc *AdminSessionUseCase) Delete(ctx context.Context, sessionID string) error {
	if err := uc.store.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}
	return nil
}

func (uc *AdminSessionUseCase) DeleteExpired(ctx context.Context) (int64, error) {
	deleted, err := uc.store.DeleteExpired(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired admin sessions: %w", err)
	}
	return deleted, nil
}
