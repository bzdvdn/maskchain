package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	domainErr "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task tenant-profile-sync#T1.3: Implement PostgresTenantRepo (AC-001, AC-005, AC-008)
type PostgresTenantRepo struct {
	pool  *pgxpool.Pool
	txMgr TransactionManager
}

func NewPostgresTenantRepo(pool *pgxpool.Pool, txMgr TransactionManager) *PostgresTenantRepo {
	return &PostgresTenantRepo{pool: pool, txMgr: txMgr}
}

func (r *PostgresTenantRepo) List(ctx context.Context) ([]*entity.Tenant, error) {
	q := getQuerier(ctx, r.pool)

	rows, err := q.Query(ctx, `
		SELECT slug, name, auth_header, api_keys, dictionaries, created_at, updated_at
		FROM tenants
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*entity.Tenant
	for rows.Next() {
		t, err := r.scanTenant(rows)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	if tenants == nil {
		return []*entity.Tenant{}, nil
	}
	return tenants, nil
}

func (r *PostgresTenantRepo) Get(ctx context.Context, slug value.TenantSlug) (*entity.Tenant, error) {
	q := getQuerier(ctx, r.pool)

	t, err := r.scanTenant(q.QueryRow(ctx, `
		SELECT slug, name, auth_header, api_keys, dictionaries, created_at, updated_at
		FROM tenants
		WHERE slug = $1`, slug.String()))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (r *PostgresTenantRepo) Create(ctx context.Context, tenant *entity.Tenant) error {
	q := getQuerier(ctx, r.pool)

	apiKeysJSON, err := json.Marshal(tenant.APIKeys())
	if err != nil {
		return fmt.Errorf("marshal api_keys: %w", err)
	}

	dictsJSON, err := marshalTenantDictionaries(tenant.Dictionaries())
	if err != nil {
		return fmt.Errorf("marshal dictionaries: %w", err)
	}

	now := time.Now().UTC()
	_, err = q.Exec(ctx, `
		INSERT INTO tenants (slug, name, auth_header, api_keys, dictionaries, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		tenant.Slug().String(),
		tenant.Name(),
		tenant.AuthHeader(),
		apiKeysJSON,
		dictsJSON,
		now,
		now,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: %s", domainErr.ErrDuplicateSlug, tenant.Slug().String())
		}
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

func (r *PostgresTenantRepo) Update(ctx context.Context, tenant *entity.Tenant) error {
	q := getQuerier(ctx, r.pool)

	apiKeysJSON, err := json.Marshal(tenant.APIKeys())
	if err != nil {
		return fmt.Errorf("marshal api_keys: %w", err)
	}

	dictsJSON, err := marshalTenantDictionaries(tenant.Dictionaries())
	if err != nil {
		return fmt.Errorf("marshal dictionaries: %w", err)
	}

	tag, err := q.Exec(ctx, `
		UPDATE tenants
		SET name = $1, auth_header = $2, api_keys = $3, dictionaries = $4, updated_at = $5
		WHERE slug = $6`,
		tenant.Name(),
		tenant.AuthHeader(),
		apiKeysJSON,
		dictsJSON,
		time.Now().UTC(),
		tenant.Slug().String(),
	)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", domainErr.ErrTenantNotFound, tenant.Slug().String())
	}
	return nil
}

func (r *PostgresTenantRepo) Delete(ctx context.Context, slug value.TenantSlug) error {
	q := getQuerier(ctx, r.pool)

	tag, err := q.Exec(ctx, `DELETE FROM tenants WHERE slug = $1`, slug.String())
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", domainErr.ErrTenantNotFound, slug.String())
	}
	return nil
}

func (r *PostgresTenantRepo) GetDictionaries(ctx context.Context, slug value.TenantSlug) ([]*dictionary.Dictionary, error) {
	q := getQuerier(ctx, r.pool)

	var dictsJSON []byte
	err := q.QueryRow(ctx, `SELECT dictionaries FROM tenants WHERE slug = $1`, slug.String()).Scan(&dictsJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", domainErr.ErrTenantNotFound, slug.String())
		}
		return nil, fmt.Errorf("get dictionaries: %w", err)
	}

	return unmarshalTenantDictionaries(dictsJSON)
}

func (r *PostgresTenantRepo) UpdateDictionaries(ctx context.Context, slug value.TenantSlug, dicts []*dictionary.Dictionary) error {
	q := getQuerier(ctx, r.pool)

	dictsJSON, err := marshalTenantDictionaries(dicts)
	if err != nil {
		return fmt.Errorf("marshal dictionaries: %w", err)
	}

	tag, err := q.Exec(ctx, `
		UPDATE tenants
		SET dictionaries = $1, updated_at = $2
		WHERE slug = $3`,
		dictsJSON,
		time.Now().UTC(),
		slug.String(),
	)
	if err != nil {
		return fmt.Errorf("update dictionaries: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", domainErr.ErrTenantNotFound, slug.String())
	}
	return nil
}

func (r *PostgresTenantRepo) scanTenant(row interface{ Scan(dest ...any) error }) (*entity.Tenant, error) {
	var slugStr, name, authHeader string
	var apiKeysJSON, dictsJSON []byte
	var createdAt, updatedAt time.Time

	err := row.Scan(&slugStr, &name, &authHeader, &apiKeysJSON, &dictsJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan tenant: %w", err)
	}

	slug, err := value.NewTenantSlug(slugStr)
	if err != nil {
		return nil, fmt.Errorf("invalid slug: %w", err)
	}

	var apiKeys []string
	if err := json.Unmarshal(apiKeysJSON, &apiKeys); err != nil {
		return nil, fmt.Errorf("unmarshal api_keys: %w", err)
	}

	dicts, err := unmarshalTenantDictionaries(dictsJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal dictionaries: %w", err)
	}

	t := entity.NewTenant(slug, name, authHeader, apiKeys, entity.WithTenantDictionaries(dicts))
	return t, nil
}

// @sk-task tenant-profile-sync#T1.3: JSONB marshal helper for tenant dictionaries
type tenantDictionaryJSON struct {
	Name      string        `json:"name"`
	MatchMode string        `json:"match_mode"`
	Entries   []interface{} `json:"entries"`
}

func marshalTenantDictionaries(dicts []*dictionary.Dictionary) ([]byte, error) {
	if dicts == nil {
		return []byte("null"), nil
	}
	items := make([]tenantDictionaryJSON, len(dicts))
	for i, d := range dicts {
		items[i] = tenantDictionaryJSON{
			Name:      d.Name(),
			MatchMode: d.MatchMode().String(),
			Entries:   d.Entries(),
		}
	}
	return json.Marshal(items)
}

func unmarshalTenantDictionaries(data []byte) ([]*dictionary.Dictionary, error) {
	if data == nil || string(data) == "null" {
		return nil, nil
	}
	var items []tenantDictionaryJSON
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("unmarshal tenant dictionaries: %w", err)
	}
	dicts := make([]*dictionary.Dictionary, len(items))
	for i, item := range items {
		slug, err := value.NewProfileSlug("tmp-" + item.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid dictionary profile slug %q: %w", item.Name, err)
		}
		dicts[i] = dictionary.NewDictionary(slug, item.Name, item.Entries, dictionary.MatchMode(item.MatchMode))
	}
	return dicts, nil
}

var _ shield.TenantRepository = (*PostgresTenantRepo)(nil)
