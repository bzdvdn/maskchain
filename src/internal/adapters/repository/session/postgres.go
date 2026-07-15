package sessionrepo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

// @sk-task sessions#T2.1: Implement PostgresSessionStore (AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-008)
type PostgresSessionStore struct {
	pool *pgxpool.Pool
}

func NewPostgresSessionStore(pool *pgxpool.Pool) *PostgresSessionStore {
	return &PostgresSessionStore{pool: pool}
}

// @sk-task sessions#T2.1: Implement Save with INSERT ON CONFLICT DO NOTHING (AC-001)
func (s *PostgresSessionStore) Save(ctx context.Context, sess *session.Session) error {
	if s.pool == nil {
		return session.ErrSessionNotFound
	}
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (id, tenant_id, model, token_count, message_count, total_masks, dict_mask_count, pii_mask_count, preprocessor_count, status, ttl, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 ON CONFLICT (id) DO NOTHING`,
		sess.SessionID, sess.TenantID, sess.Model,
		sess.TokenCount, sess.MessageCount, sess.TotalMasks, sess.DictMaskCount, sess.PIIMaskCount, sess.PreprocessorCount,
		string(sess.Status), sess.TTL, sess.CreatedAt, sess.ExpiresAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return session.ErrSessionConflict
	}
	return nil
}

// @sk-task sessions#T2.1: Implement Get with tenant-scoped SELECT (AC-003)
func (s *PostgresSessionStore) Get(ctx context.Context, tenantID, sessionID string) (*session.Session, error) {
	if s.pool == nil {
		return nil, session.ErrSessionNotFound
	}
	var sess session.Session
	var statusStr string
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, model, token_count, message_count, total_masks, dict_mask_count, pii_mask_count, preprocessor_count, status, ttl, created_at, expires_at
		 FROM sessions WHERE id = $1 AND tenant_id = $2`,
		sessionID, tenantID).Scan(
		&sess.SessionID, &sess.TenantID, &sess.Model,
		&sess.TokenCount, &sess.MessageCount, &sess.TotalMasks, &sess.DictMaskCount, &sess.PIIMaskCount, &sess.PreprocessorCount,
		&statusStr, &sess.TTL, &sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, session.ErrSessionNotFound
		}
		return nil, err
	}
	sess.Status = session.SessionStatus(statusStr)
	return &sess, nil
}

// @sk-task sessions#T2.1: Implement IncrementCounts with atomic UPDATE ... + (AC-002)
func (s *PostgresSessionStore) IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error {
	if s.pool == nil {
		return session.ErrSessionNotFound
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE sessions SET
			token_count = token_count + $1,
			message_count = message_count + $2,
			total_masks = total_masks + $3,
			dict_mask_count = dict_mask_count + $4,
			pii_mask_count = pii_mask_count + $5,
			preprocessor_count = preprocessor_count + $6
		 WHERE id = $7 AND tenant_id = $8 AND status = 'active' AND expires_at > NOW()`,
		tokens, messages, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount, sessionID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}

// @sk-task sessions#T2.1: Implement ExtendTTL (AC-005)
func (s *PostgresSessionStore) ExtendTTL(ctx context.Context, tenantID, sessionID string, newExpiresAt time.Time) error {
	if s.pool == nil {
		return session.ErrSessionNotFound
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE sessions SET expires_at = $1 WHERE id = $2 AND tenant_id = $3 AND status = 'active'`,
		newExpiresAt, sessionID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}

// @sk-task sessions#T2.1: Implement Close (AC-006)
func (s *PostgresSessionStore) Close(ctx context.Context, tenantID, sessionID string) error {
	if s.pool == nil {
		return session.ErrSessionNotFound
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE sessions SET status = 'closed' WHERE id = $1 AND tenant_id = $2 AND status = 'active'`,
		sessionID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}

// @sk-task sessions#T2.1: Implement DeleteExpired (AC-007)
func (s *PostgresSessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	if s.pool == nil {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM sessions WHERE status = 'expired' OR expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// @sk-task sessions#T2.1: Implement ListByTenant with pagination (AC-004)
func (s *PostgresSessionStore) ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*session.ListResult, error) {
	if s.pool == nil {
		return &session.ListResult{Items: []session.Session{}, Total: 0, Page: int(page), Limit: int(limit)}, nil
	}
	offset := (page - 1) * limit

	var total int32
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE tenant_id = $1`, tenantID).Scan(&total)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, model, token_count, message_count, total_masks, dict_mask_count, pii_mask_count, preprocessor_count, status, ttl, created_at, expires_at
		 FROM sessions WHERE tenant_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []session.Session
	for rows.Next() {
		var sess session.Session
		var statusStr string
		if err := rows.Scan(
			&sess.SessionID, &sess.TenantID, &sess.Model,
			&sess.TokenCount, &sess.MessageCount, &sess.TotalMasks, &sess.DictMaskCount, &sess.PIIMaskCount, &sess.PreprocessorCount,
			&statusStr, &sess.TTL, &sess.CreatedAt, &sess.ExpiresAt); err != nil {
			return nil, err
		}
		sess.Status = session.SessionStatus(statusStr)
		items = append(items, sess)
	}
	if items == nil {
		items = []session.Session{}
	}

	return &session.ListResult{
		Items: items,
		Total: int(total),
		Page:  int(page),
		Limit: int(limit),
	}, nil
}
