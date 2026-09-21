package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// @sk-task 300-virtual-keys#T1.3: PostgresVirtualKeyRepository (AC-001)
//
// PostgresVirtualKeyRepository persists virtual keys with SHA-256 hashes and
// soft-delete revocation.
type PostgresVirtualKeyRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresVirtualKeyRepository(pool *pgxpool.Pool) *PostgresVirtualKeyRepository {
	return &PostgresVirtualKeyRepository{pool: pool}
}

const virtualKeySelect = `
	SELECT id, key_hash, tenant_id, label, allowed_models, blocked_models,
	       budget_cap, spent, expires_at, metadata, enabled, created_at, updated_at
	FROM virtual_keys`

// @sk-task 300-virtual-keys#T1.3: FindByKeyHash resolves a key by hash (AC-001)
func (r *PostgresVirtualKeyRepository) FindByKeyHash(ctx context.Context, hash string) (*virtualkey.VirtualKey, error) {
	row := r.pool.QueryRow(ctx, virtualKeySelect+` WHERE key_hash = $1 AND enabled = TRUE`, hash)
	return scanVirtualKey(row)
}

// @sk-task 300-virtual-keys#T1.3: GetById returns a key by public id (AC-001)
func (r *PostgresVirtualKeyRepository) GetById(ctx context.Context, id string) (*virtualkey.VirtualKey, error) {
	row := r.pool.QueryRow(ctx, virtualKeySelect+` WHERE id = $1`, id)
	return scanVirtualKey(row)
}

// @sk-task 300-virtual-keys#T1.3: ListByTenant returns all keys of a tenant (AC-001)
func (r *PostgresVirtualKeyRepository) ListByTenant(ctx context.Context, tenantID string) ([]*virtualkey.VirtualKey, error) {
	rows, err := r.pool.Query(ctx, virtualKeySelect+` WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list virtual keys: %w", err)
	}
	defer rows.Close()
	return collectVirtualKeys(rows)
}

// @sk-task 300-virtual-keys#T1.3: List returns all keys (AC-001)
func (r *PostgresVirtualKeyRepository) List(ctx context.Context) ([]*virtualkey.VirtualKey, error) {
	rows, err := r.pool.Query(ctx, virtualKeySelect+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list virtual keys: %w", err)
	}
	defer rows.Close()
	return collectVirtualKeys(rows)
}

// @sk-task ui-production-readiness#T7.1: DB-level pagination/search (AC-008)
func (r *PostgresVirtualKeyRepository) ListPaged(ctx context.Context, limit, offset int, search string) ([]*virtualkey.VirtualKey, int, error) {
	pattern := "%" + search + "%"

	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM virtual_keys
		WHERE $1 = '' OR id ILIKE $2 OR tenant_id ILIKE $2 OR label ILIKE $2`, search, pattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count virtual keys: %w", err)
	}

	rows, err := r.pool.Query(ctx, virtualKeySelect+`
		WHERE $1 = '' OR id ILIKE $2 OR tenant_id ILIKE $2 OR label ILIKE $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`, search, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list virtual keys: %w", err)
	}
	defer rows.Close()

	keys, err := collectVirtualKeys(rows)
	if err != nil {
		return nil, 0, err
	}
	return keys, total, nil
}

// @sk-task 300-virtual-keys#T1.3: Create persists a new virtual key (AC-001)
func (r *PostgresVirtualKeyRepository) Create(ctx context.Context, key *virtualkey.VirtualKey) error {
	allowed, err := marshalStringSlice(key.AllowedModels)
	if err != nil {
		return err
	}
	blocked, err := marshalStringSlice(key.BlockedModels)
	if err != nil {
		return err
	}
	meta, err := json.Marshal(key.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO virtual_keys (id, key_hash, tenant_id, label, allowed_models, blocked_models,
		                          budget_cap, spent, expires_at, metadata, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		key.ID, key.KeyHash, key.TenantID, key.Label, allowed, blocked,
		key.BudgetCap, key.Spent, key.ExpiresAt, meta, key.Enabled,
		key.CreatedAt, key.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create virtual key: %w", err)
	}
	return nil
}

// @sk-task 300-virtual-keys#T1.3: Update persists changes to an existing key (AC-001)
func (r *PostgresVirtualKeyRepository) Update(ctx context.Context, key *virtualkey.VirtualKey) error {
	allowed, err := marshalStringSlice(key.AllowedModels)
	if err != nil {
		return err
	}
	blocked, err := marshalStringSlice(key.BlockedModels)
	if err != nil {
		return err
	}
	meta, err := json.Marshal(key.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE virtual_keys
		SET label = $2, allowed_models = $3, blocked_models = $4, budget_cap = $5,
		    spent = $6, expires_at = $7, metadata = $8, enabled = $9, updated_at = $10
		WHERE id = $1`,
		key.ID, key.Label, allowed, blocked, key.BudgetCap, key.Spent,
		key.ExpiresAt, meta, key.Enabled, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update virtual key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return virtualkey.ErrNotFound
	}
	return nil
}

// @sk-task 300-virtual-keys#T1.3: Delete soft-deletes (revokes) a key (AC-001)
func (r *PostgresVirtualKeyRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE virtual_keys SET enabled = FALSE, updated_at = now() WHERE id = $1 AND enabled = TRUE`, id)
	if err != nil {
		return fmt.Errorf("revoke virtual key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return virtualkey.ErrNotFound
	}
	return nil
}

// @sk-task 300-virtual-keys#T1.4: BackfillFromLegacy creates keys for raw tenant api keys (AC-004)
func (r *PostgresVirtualKeyRepository) BackfillFromLegacy(ctx context.Context, legacy map[string][]string) (int, error) {
	created := 0
	for tenantID, keys := range legacy {
		for _, raw := range keys {
			if raw == "" {
				continue
			}
			hash := virtualkey.KeyHash(raw)
			var existing string
			err := r.pool.QueryRow(ctx, `SELECT id FROM virtual_keys WHERE key_hash = $1`, hash).Scan(&existing)
			if err == nil {
				continue
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return created, fmt.Errorf("check virtual key: %w", err)
			}
			id, err := virtualkey.NewKeyID()
			if err != nil {
				return created, err
			}
			now := time.Now().UTC()
			key := &virtualkey.VirtualKey{
				ID:        id,
				TenantID:  tenantID,
				KeyHash:   hash,
				Label:     "legacy",
				Metadata:  map[string]string{},
				Enabled:   true,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := r.Create(ctx, key); err != nil {
				return created, err
			}
			created++
		}
	}
	return created, nil
}

func marshalStringSlice(items []string) ([]byte, error) {
	b, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshal string list: %w", err)
	}
	return b, nil
}

func collectVirtualKeys(rows pgx.Rows) ([]*virtualkey.VirtualKey, error) {
	var out []*virtualkey.VirtualKey
	for rows.Next() {
		key, err := scanVirtualKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	if out == nil {
		return []*virtualkey.VirtualKey{}, nil
	}
	return out, nil
}

func scanVirtualKey(row interface{ Scan(dest ...any) error }) (*virtualkey.VirtualKey, error) {
	var (
		id, hash, tenantID, label string
		allowedRaw, blockedRaw    []byte
		budgetCap, spent          *float64
		expiresAt                 *time.Time
		metaRaw                   []byte
		enabled                   bool
		createdAt, updatedAt      time.Time
	)
	err := row.Scan(&id, &hash, &tenantID, &label, &allowedRaw, &blockedRaw,
		&budgetCap, &spent, &expiresAt, &metaRaw, &enabled, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, virtualkey.ErrNotFound
		}
		return nil, fmt.Errorf("scan virtual key: %w", err)
	}

	allowed, err := unmarshalStringSlice(allowedRaw)
	if err != nil {
		return nil, err
	}
	blocked, err := unmarshalStringSlice(blockedRaw)
	if err != nil {
		return nil, err
	}
	metadata := map[string]string{}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	key := &virtualkey.VirtualKey{
		ID:            id,
		KeyHash:       hash,
		TenantID:      tenantID,
		Label:         label,
		AllowedModels: allowed,
		BlockedModels: blocked,
		BudgetCap:     budgetCap,
		Spent:         spentVal(spent),
		ExpiresAt:     expiresAt,
		Metadata:      metadata,
		Enabled:       enabled,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	return key, nil
}

func unmarshalStringSlice(raw []byte) ([]string, error) {
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal string list: %w", err)
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}

func spentVal(spent *float64) float64 {
	if spent == nil {
		return 0
	}
	return *spent
}

var _ virtualkey.VirtualKeyRepository = (*PostgresVirtualKeyRepository)(nil)
