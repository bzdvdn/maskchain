package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 30-shield-persistence#T2.3: Implement PostgresProfileRepo (DM-001)
type PostgresProfileRepo struct {
	pool     *pgxpool.Pool
	dictRepo *PostgresDictionaryRepo
	txMgr    TransactionManager
}

func NewPostgresProfileRepo(pool *pgxpool.Pool, dictRepo *PostgresDictionaryRepo, txMgr TransactionManager) *PostgresProfileRepo {
	return &PostgresProfileRepo{pool: pool, dictRepo: dictRepo, txMgr: txMgr}
}

func (r *PostgresProfileRepo) Save(ctx context.Context, profile *entity.Profile) error {
	q := getQuerier(ctx, r.pool)

	ppJSON, err := marshalPreprocessors(profile.Preprocessors())
	if err != nil {
		return fmt.Errorf("marshal preprocessors: %w", err)
	}

	status := "active"
	if !profile.Enabled() {
		status = "disabled"
	}

	var pp *[]byte
	if ppJSON != nil && string(ppJSON) != "null" {
		pp = &ppJSON
	}

	_, err = q.Exec(ctx, `
		INSERT INTO profiles (id, slug, name, tenant_id, preprocessors, status, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8)
		ON CONFLICT (slug) DO UPDATE SET
			name = EXCLUDED.name,
			preprocessors = EXCLUDED.preprocessors,
			status = EXCLUDED.status,
			version = profiles.version + 1,
			updated_at = EXCLUDED.updated_at`,
		profile.ID().String(),
		profile.Slug().String(),
		profile.Name(),
		profile.TenantID().String(),
		pp,
		status,
		profile.CreatedAt(),
		profile.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("save profile: %w", err)
	}

	for _, dict := range profile.Dictionaries() {
		if err := r.dictRepo.Save(ctx, dict); err != nil {
			return fmt.Errorf("save dictionary %q: %w", dict.Name(), err)
		}
	}

	return nil
}

func (r *PostgresProfileRepo) FindBySlug(ctx context.Context, tenantID value.TenantID, slug value.ProfileSlug) (*entity.Profile, error) {
	q := getQuerier(ctx, r.pool)

	var idStr, name, status string
	var ppJSON []byte
	var createdAt, updatedAt time.Time

	err := q.QueryRow(ctx, `
		SELECT id, name, preprocessors, status, created_at, updated_at
		FROM profiles
		WHERE slug = $1 AND tenant_id = $2`,
		slug.String(), tenantID.String()).Scan(&idStr, &name, &ppJSON, &status, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find profile by slug: %w", err)
	}

	return r.buildProfile(ctx, idStr, slug.String(), tenantID, name, ppJSON, status, createdAt, updatedAt)
}

func (r *PostgresProfileRepo) FindByID(ctx context.Context, id value.ProfileID) (*entity.Profile, error) {
	q := getQuerier(ctx, r.pool)

	var slugStr, name, tenantStr, status string
	var ppJSON []byte
	var createdAt, updatedAt time.Time

	err := q.QueryRow(ctx, `
		SELECT slug, name, tenant_id, preprocessors, status, created_at, updated_at
		FROM profiles
		WHERE id = $1`,
		id.String()).Scan(&slugStr, &name, &tenantStr, &ppJSON, &status, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find profile by id: %w", err)
	}

	slug, err := value.NewProfileSlug(slugStr)
	if err != nil {
		return nil, fmt.Errorf("invalid slug: %w", err)
	}
	tenantID, err := value.NewTenantID(tenantStr)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant id: %w", err)
	}

	return r.buildProfile(ctx, id.String(), slug.String(), tenantID, name, ppJSON, status, createdAt, updatedAt)
}

func (r *PostgresProfileRepo) ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Profile, error) {
	q := getQuerier(ctx, r.pool)

	rows, err := q.Query(ctx, `
		SELECT id, slug, name, preprocessors, status, created_at, updated_at
		FROM profiles
		WHERE tenant_id = $1
		ORDER BY created_at DESC`,
		tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("list profiles by tenant: %w", err)
	}
	defer rows.Close()

	var profiles []*entity.Profile
	for rows.Next() {
		var idStr, slugStr, name, status string
		var ppJSON []byte
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&idStr, &slugStr, &name, &ppJSON, &status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}

		profile, err := r.buildProfile(ctx, idStr, slugStr, tenantID, name, ppJSON, status, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if profiles == nil {
		return []*entity.Profile{}, nil
	}
	return profiles, nil
}

// @sk-task 30-shield-persistence#T3.2: Implement cascade delete in transaction (AC-002, DEC-005)
func (r *PostgresProfileRepo) Delete(ctx context.Context, id value.ProfileID) error {
	return r.txMgr.RunInTx(ctx, func(txCtx context.Context) error {
		q := getQuerier(txCtx, r.pool)

		_, err := q.Exec(txCtx, `DELETE FROM incidents WHERE profile_slug IN (SELECT slug FROM profiles WHERE id = $1)`, id.String())
		if err != nil {
			return fmt.Errorf("delete incidents: %w", err)
		}

		_, err = q.Exec(txCtx, `DELETE FROM dictionary_entries WHERE profile_slug IN (SELECT slug FROM profiles WHERE id = $1)`, id.String())
		if err != nil {
			return fmt.Errorf("delete dictionary entries: %w", err)
		}

		tag, err := q.Exec(txCtx, `DELETE FROM profiles WHERE id = $1`, id.String())
		if err != nil {
			return fmt.Errorf("delete profile: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		return nil
	})
}

func (r *PostgresProfileRepo) buildProfile(
	ctx context.Context,
	idStr, slugStr string,
	tenantID value.TenantID,
	name string,
	ppJSON []byte,
	status string,
	createdAt, updatedAt time.Time,
) (*entity.Profile, error) {
	pid, err := value.NewProfileID(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid profile id: %w", err)
	}

	slug, err := value.NewProfileSlug(slugStr)
	if err != nil {
		return nil, fmt.Errorf("invalid slug: %w", err)
	}

	preprocessors, err := unmarshalPreprocessors(ppJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal preprocessors: %w", err)
	}

	dict, err := r.dictRepo.FindByProfileSlug(ctx, slugStr)
	if err != nil {
		return nil, fmt.Errorf("load dictionaries: %w", err)
	}

	var dicts []*dictionary.Dictionary
	if dict != nil {
		dicts = []*dictionary.Dictionary{dict}
	}

	enabled := strings.ToLower(status) != "disabled"

	p := entity.NewProfile(pid, slug, tenantID, name,
		entity.WithPreprocessors(preprocessors),
		entity.WithDictionaries(dicts),
		entity.WithEnabled(enabled),
	)
	return p, nil
}

// @sk-task 30-shield-persistence#T2.3: Marshal preprocessors to JSONB
func marshalPreprocessors(pp []preprocessor.PreprocessorDef) ([]byte, error) {
	if pp == nil {
		return []byte("null"), nil
	}
	return json.Marshal(pp)
}

// @sk-task 30-shield-persistence#T2.3: Unmarshal preprocessors from JSONB
func unmarshalPreprocessors(data []byte) ([]preprocessor.PreprocessorDef, error) {
	if data == nil || string(data) == "null" {
		return nil, nil
	}
	var pp []preprocessor.PreprocessorDef
	if err := json.Unmarshal(data, &pp); err != nil {
		return nil, err
	}
	return pp, nil
}

var _ shield.ProfileRepository = (*PostgresProfileRepo)(nil)
