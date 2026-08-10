package conversationrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
)

// @sk-task conversation-logging#T1.5: Implement PgConversationStore (AC-001, AC-004, AC-006, AC-007)
//
// PgConversationStore persists encrypted conversation logs to PostgreSQL.
type PgConversationStore struct {
	pool *pgxpool.Pool
}

// @sk-task conversation-logging#T1.5: NewPgConversationStore creates the store (AC-001)
func NewPgConversationStore(pool *pgxpool.Pool) *PgConversationStore {
	return &PgConversationStore{pool: pool}
}

const insertConversationLogSQL = `
	INSERT INTO conversation_logs
		(id, tenant_id, model, status, masked, streamed, mask_id, masking, request, response, request_len, response_len, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	ON CONFLICT (id) DO NOTHING`

// @sk-task conversation-logging#T2.1: SaveBatch inserts encrypted logs via pgx.Batch (AC-001, AC-003)
// @sk-task conversation-logging#T2.4: Persist Streamed flag (RQ-009, DEC-007, AC-003)
func (s *PgConversationStore) SaveBatch(ctx context.Context, logs []conversation.ConversationLog) error {
	if s.pool == nil || len(logs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, l := range logs {
		var masking any
		if l.Masked {
			masking = l.Masking
		}
		var response any
		if l.Response != nil {
			response = l.Response
		}
		var maskID any
		if l.MaskID != "" {
			maskID = l.MaskID
		}
		batch.Queue(insertConversationLogSQL, l.ID, l.TenantID, l.Model,
			string(l.Status), l.Masked, l.Streamed, maskID, masking, l.Request, response, l.RequestLen, l.ResponseLen, l.CreatedAt)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range logs {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// @sk-task conversation-logging#T1.5: DeleteOlderThan removes records for TTL cleanup (AC-007)
func (s *PgConversationStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if s.pool == nil {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM conversation_logs WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// @sk-task conversation-logging#T1.5: List returns metadata-only records with pagination and tenant filter (AC-006)
// @sk-task conversation-logging#T2.4: Surface Streamed flag in list (RQ-009, DEC-007, AC-003)
func (s *PgConversationStore) List(ctx context.Context, filter conversation.ConversationFilter) (conversation.ConversationPage, error) {
	if s.pool == nil {
		return conversation.ConversationPage{}, nil
	}
	perPage := filter.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage

	var tenantFilter string
	var args []any
	args = append(args, perPage, offset)
	if filter.TenantID != "" {
		tenantFilter = `WHERE tenant_id = $3`
		args = append(args, filter.TenantID)
	}

	rows, err := s.pool.Query(ctx,
		fmt.Sprintf(`SELECT id, tenant_id, model, status, masked, streamed, mask_id, created_at
		 FROM conversation_logs %s ORDER BY created_at DESC LIMIT $1 OFFSET $2`, tenantFilter), args...)
	if err != nil {
		return conversation.ConversationPage{}, err
	}
	defer rows.Close()

	var items []conversation.ConversationLog
	for rows.Next() {
		var l conversation.ConversationLog
		var maskID *string
		if err := rows.Scan(&l.ID, &l.TenantID, &l.Model, &l.Status, &l.Masked, &l.Streamed, &maskID, &l.CreatedAt); err != nil {
			return conversation.ConversationPage{}, err
		}
		if maskID != nil {
			l.MaskID = *maskID
		}
		items = append(items, l)
	}
	if err := rows.Err(); err != nil {
		return conversation.ConversationPage{}, err
	}

	var total int
	if filter.TenantID != "" {
		if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM conversation_logs WHERE tenant_id = $1`,
			filter.TenantID).Scan(&total); err != nil {
			return conversation.ConversationPage{}, err
		}
	} else if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM conversation_logs`).Scan(&total); err != nil {
		return conversation.ConversationPage{}, err
	}

	return conversation.ConversationPage{Items: items, Total: total}, nil
}

// @sk-task conversation-logging#T2.1: Get returns a single record including encrypted content (AC-005)
// @sk-task conversation-logging#T2.4: Read Streamed flag in detail (RQ-009, DEC-007, AC-003)
func (s *PgConversationStore) Get(ctx context.Context, id string) (*conversation.ConversationLog, error) {
	if s.pool == nil {
		return nil, conversation.ErrNotFound
	}
	var l conversation.ConversationLog
	var masking []byte
	var response []byte
	var maskID *string
	if err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, model, status, masked, streamed, mask_id, masking, request, response, request_len, response_len, created_at
		 FROM conversation_logs WHERE id = $1`, id).
		Scan(&l.ID, &l.TenantID, &l.Model, &l.Status, &l.Masked, &l.Streamed,
			&maskID, &masking, &l.Request, &response, &l.RequestLen, &l.ResponseLen, &l.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, conversation.ErrNotFound
		}
		return nil, err
	}
	if maskID != nil {
		l.MaskID = *maskID
	}
	l.WithMasking(masking)
	if len(response) > 0 {
		l.Response = response
		l.ResponseLen = len(response)
	}
	return &l, nil
}

var _ conversation.ConversationStore = (*PgConversationStore)(nil)
