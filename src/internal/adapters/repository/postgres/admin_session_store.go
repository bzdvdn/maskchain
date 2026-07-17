package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/admin_session"
)

// @sk-task admin-ui-design#T1.4: Postgres admin session store (AC-001, AC-004)
type PostgresAdminSessionStore struct {
	pool *pgxpool.Pool
}

func NewPostgresAdminSessionStore(pool *pgxpool.Pool) *PostgresAdminSessionStore {
	return &PostgresAdminSessionStore{pool: pool}
}

func (s *PostgresAdminSessionStore) Save(ctx context.Context, sess *admin_session.AdminSession) error {
	q := getQuerier(ctx, s.pool)
	_, err := q.Exec(ctx, `
		INSERT INTO admin_sessions (id, username, token_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		sess.ID, sess.Username, sess.TokenHash, sess.CreatedAt, sess.ExpiresAt)
	if err != nil {
		return fmt.Errorf("save admin session: %w", err)
	}
	return nil
}

func (s *PostgresAdminSessionStore) GetByTokenHash(ctx context.Context, tokenHash string) (*admin_session.AdminSession, error) {
	q := getQuerier(ctx, s.pool)
	row := q.QueryRow(ctx, `
		SELECT id, username, token_hash, created_at, expires_at
		FROM admin_sessions
		WHERE token_hash = $1`, tokenHash)

	var sess admin_session.AdminSession
	if err := row.Scan(&sess.ID, &sess.Username, &sess.TokenHash, &sess.CreatedAt, &sess.ExpiresAt); err != nil {
		if err.Error() == "no rows in result set" {
			return nil, admin_session.ErrSessionNotFound
		}
		return nil, fmt.Errorf("get admin session by token hash: %w", err)
	}
	return &sess, nil
}

func (s *PostgresAdminSessionStore) Delete(ctx context.Context, sessionID string) error {
	q := getQuerier(ctx, s.pool)
	_, err := q.Exec(ctx, `DELETE FROM admin_sessions WHERE id = $1`, sessionID)
	if err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}
	return nil
}

func (s *PostgresAdminSessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	q := getQuerier(ctx, s.pool)
	tag, err := q.Exec(ctx, `DELETE FROM admin_sessions WHERE expires_at < now()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired admin sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

func NewAdminCleanupWorker(store *PostgresAdminSessionStore, interval time.Duration) *adminCleanupWorker {
	return &adminCleanupWorker{store: store, interval: interval}
}

type adminCleanupWorker struct {
	store    *PostgresAdminSessionStore
	interval time.Duration
}

func (w *adminCleanupWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.store.DeleteExpired(ctx)
		}
	}
}
