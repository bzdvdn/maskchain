package sessionrepo

import (
	"context"
	"log/slog"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

// @sk-task sessions#T3.2: Implement CachedSessionStore decorator (AC-008)
type CachedSessionStore struct {
	primary   session.SessionStore
	secondary session.SessionStore
	log       *slog.Logger
}

func NewCachedSessionStore(primary, secondary session.SessionStore, log *slog.Logger) *CachedSessionStore {
	return &CachedSessionStore{
		primary:   primary,
		secondary: secondary,
		log:       log,
	}
}

func (s *CachedSessionStore) secondaryOp(ctx context.Context, op string, fn func() error) {
	if err := fn(); err != nil {
		s.log.Warn("session cache secondary operation failed", slog.String("op", op), slog.String("error", err.Error()))
	}
}

// @sk-task sessions#T3.2: Save sync PG + best-effort Valkey (AC-008)
func (s *CachedSessionStore) Save(ctx context.Context, sess *session.Session) error {
	if err := s.primary.Save(ctx, sess); err != nil {
		return err
	}
	s.secondaryOp(ctx, "Save", func() error {
		return s.secondary.Save(ctx, sess)
	})
	return nil
}

// @sk-task sessions#T3.2: Get cache-first -> miss -> PG -> backfill Valkey (AC-008)
func (s *CachedSessionStore) Get(ctx context.Context, tenantID, sessionID string) (*session.Session, error) {
	sess, err := s.secondary.Get(ctx, tenantID, sessionID)
	if err == nil {
		return sess, nil
	}
	if err != session.ErrSessionNotFound {
		s.secondaryOp(ctx, "Get", func() error { return err })
	}

	sess, err = s.primary.Get(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}

	s.secondaryOp(ctx, "Save", func() error {
		return s.secondary.Save(ctx, sess)
	})
	return sess, nil
}

// @sk-task sessions#T3.2: IncrementCounts sync PG + best-effort Valkey (AC-008)
func (s *CachedSessionStore) IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error {
	if err := s.primary.IncrementCounts(ctx, tenantID, sessionID, tokens, messages, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount); err != nil {
		return err
	}
	s.secondaryOp(ctx, "IncrementCounts", func() error {
		updated, err := s.primary.Get(ctx, tenantID, sessionID)
		if err != nil {
			return err
		}
		return s.secondary.Save(ctx, updated)
	})
	return nil
}

// @sk-task sessions#T3.2: ExtendTTL sync PG + best-effort Valkey (AC-008)
func (s *CachedSessionStore) ExtendTTL(ctx context.Context, tenantID, sessionID string, newExpiresAt time.Time) error {
	if err := s.primary.ExtendTTL(ctx, tenantID, sessionID, newExpiresAt); err != nil {
		return err
	}
	s.secondaryOp(ctx, "ExtendTTL", func() error {
		updated, err := s.primary.Get(ctx, tenantID, sessionID)
		if err != nil {
			return err
		}
		return s.secondary.Save(ctx, updated)
	})
	return nil
}

// @sk-task sessions#T3.2: Close sync PG + best-effort Valkey invalidate (AC-008)
func (s *CachedSessionStore) Close(ctx context.Context, tenantID, sessionID string) error {
	if err := s.primary.Close(ctx, tenantID, sessionID); err != nil {
		return err
	}
	s.secondaryOp(ctx, "Close", func() error {
		updated, err := s.primary.Get(ctx, tenantID, sessionID)
		if err != nil {
			return err
		}
		return s.secondary.Save(ctx, updated)
	})
	return nil
}

func (s *CachedSessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	return s.primary.DeleteExpired(ctx)
}

func (s *CachedSessionStore) ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*session.ListResult, error) {
	return s.primary.ListByTenant(ctx, tenantID, page, limit)
}

func (s *CachedSessionStore) ListAll(ctx context.Context, page, limit int32) (*session.ListResult, error) {
	return s.primary.ListAll(ctx, page, limit)
}
