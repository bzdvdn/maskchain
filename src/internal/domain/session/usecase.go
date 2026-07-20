package session

import (
	"context"
	"fmt"
	"time"
)

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 100
)

// @sk-task sessions#T1.2: Implement SessionUseCase with all methods (AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-010)
//
// SessionUseCase represents a domain entity or configuration.
type SessionUseCase struct {
	store SessionStore
}

func NewSessionUseCase(store SessionStore) *SessionUseCase {
	return &SessionUseCase{store: store}
}

func (uc *SessionUseCase) Create(ctx context.Context, sessionID, tenantID, model string, ttl time.Duration) (*Session, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	if sessionID == "" {
		var genErr error
		sessionID, genErr = NewSessionID()
		if genErr != nil {
			return nil, fmt.Errorf("generate session id: %w", genErr)
		}
	}
	now := time.Now()
	s := &Session{
		SessionID: sessionID,
		TenantID:  tenantID,
		Model:     model,
		Status:    SessionStatusActive,
		TTL:       ttl,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	if err := uc.store.Save(ctx, s); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}
	return s, nil
}

// @sk-task sessions#T1.2: Implement Get with tenant-scoped check (AC-003)
func (uc *SessionUseCase) Get(ctx context.Context, tenantID, sessionID string) (*Session, error) {
	s, err := uc.store.Get(ctx, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if s.Status == SessionStatusActive && time.Now().After(s.ExpiresAt) {
		s.Status = SessionStatusExpired
	}
	return s, nil
}

// @sk-task sessions#T1.2: Implement IncrementCounts (AC-002)
func (uc *SessionUseCase) IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error {
	if err := uc.store.IncrementCounts(ctx, tenantID, sessionID, tokens, messages, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount); err != nil {
		return fmt.Errorf("increment counts: %w", err)
	}
	return nil
}

// @sk-task sessions#T1.2: Implement ExtendTTL (AC-005)
func (uc *SessionUseCase) ExtendTTL(ctx context.Context, tenantID, sessionID string, ttlSeconds int32) (*Session, error) {
	s, err := uc.store.Get(ctx, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("extend TTL: %w", err)
	}
	switch s.Status {
	case SessionStatusExpired:
		return nil, ErrSessionExpired
	case SessionStatusClosed:
		return nil, ErrSessionClosed
	}
	newExpiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	if err := uc.store.ExtendTTL(ctx, tenantID, sessionID, newExpiresAt); err != nil {
		return nil, fmt.Errorf("extend TTL: %w", err)
	}
	s.ExpiresAt = newExpiresAt
	return s, nil
}

// @sk-task sessions#T1.2: Implement Close (AC-006)
func (uc *SessionUseCase) Close(ctx context.Context, tenantID, sessionID string) error {
	s, err := uc.store.Get(ctx, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	switch s.Status {
	case SessionStatusExpired:
		return ErrSessionExpired
	case SessionStatusClosed:
		return ErrSessionClosed
	}
	if err := uc.store.Close(ctx, tenantID, sessionID); err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	return nil
}

// @sk-task sessions#T1.2: Implement ListByTenant with pagination (AC-004)
func (uc *SessionUseCase) ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*ListResult, error) {
	if page < 1 {
		page = defaultPage
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	result, err := uc.store.ListByTenant(ctx, tenantID, page, limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return result, nil
}

// @sk-task admin-ui-design#T4.4: ListAll returns all sessions (admin use)
func (uc *SessionUseCase) ListAll(ctx context.Context, page, limit int32) (*ListResult, error) {
	if page < 1 {
		page = defaultPage
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	result, err := uc.store.ListAll(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("list all sessions: %w", err)
	}
	return result, nil
}

// @sk-task sessions#T1.2: Implement DeleteExpired (AC-007)
func (uc *SessionUseCase) DeleteExpired(ctx context.Context) (int64, error) {
	deleted, err := uc.store.DeleteExpired(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired: %w", err)
	}
	return deleted, nil
}
