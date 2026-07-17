package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// @sk-task admin-ui-design#T1.4: Audit log entry (AC-005)
type AuditLogEntry struct {
	ID            int64           `json:"id"`
	AdminUsername string          `json:"admin_username"`
	Action        string          `json:"action"`
	Target        string          `json:"target"`
	Details       json.RawMessage `json:"details,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// @sk-task admin-ui-design#T1.4: AuditLogStore with async buffered channel + flush worker (AC-005)
type AuditLogStore struct {
	pool    *pgxpool.Pool
	ch      chan *AuditLogEntry
	wg      sync.WaitGroup
	closeCh chan struct{}
}

func NewAuditLogStore(pool *pgxpool.Pool, bufSize int) *AuditLogStore {
	s := &AuditLogStore{
		pool:    pool,
		ch:      make(chan *AuditLogEntry, bufSize),
		closeCh: make(chan struct{}),
	}
	s.wg.Add(1)
	go s.flushLoop()
	return s
}

func (s *AuditLogStore) Write(ctx context.Context, entry *AuditLogEntry) error {
	select {
	case s.ch <- entry:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("audit log buffer full")
	}
}

func (s *AuditLogStore) flush() {
	var batch []*AuditLogEntry
	for {
		select {
		case entry := <-s.ch:
			batch = append(batch, entry)
		default:
			if len(batch) == 0 {
				return
			}
			s.insertBatch(context.Background(), batch)
			batch = nil
		}
	}
}

func (s *AuditLogStore) flushLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.closeCh:
			s.flush()
			return
		case <-ticker.C:
			s.flush()
		}
	}
}

func (s *AuditLogStore) insertBatch(ctx context.Context, entries []*AuditLogEntry) {
	if len(entries) == 0 {
		return
	}
	q := getQuerier(ctx, s.pool)
	for _, e := range entries {
		var details any
		if e.Details != nil {
			details = string(e.Details)
		}
		_, err := q.Exec(ctx, `
			INSERT INTO audit_log (admin_username, action, target, details, created_at)
			VALUES ($1, $2, $3, $4, $5)`,
			e.AdminUsername, e.Action, e.Target, details, e.CreatedAt)
		if err != nil {
			// log and skip — audit is best-effort
			fmt.Printf("audit log insert error: %v\n", err)
		}
	}
}

func (s *AuditLogStore) List(ctx context.Context, limit, offset int) ([]AuditLogEntry, error) {
	q := getQuerier(ctx, s.pool)
	rows, err := q.Query(ctx, `
		SELECT id, admin_username, action, target, details, created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list audit log: %w", err)
	}
	defer rows.Close()

	var entries []AuditLogEntry
	for rows.Next() {
		var e AuditLogEntry
		var details any
		if err := rows.Scan(&e.ID, &e.AdminUsername, &e.Action, &e.Target, &details, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		if details != nil {
			if s, ok := details.(string); ok {
				e.Details = json.RawMessage(s)
			}
		}
		entries = append(entries, e)
	}
	if entries == nil {
		return []AuditLogEntry{}, nil
	}
	return entries, nil
}

func (s *AuditLogStore) Shutdown() {
	close(s.closeCh)
	s.wg.Wait()
}
