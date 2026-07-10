package profilerepo

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 25-shield-preprocessors#T3.1: Add preprocessors JSONB field (DM-001)
func marshalPreprocessors(pp []preprocessor.PreprocessorDef) ([]byte, error) {
	if pp == nil {
		return []byte("null"), nil
	}
	return json.Marshal(pp)
}

// @sk-task 25-shield-preprocessors#T3.1: Add preprocessors JSONB field (DM-001)
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

// @sk-task 24-shield-dictionaries#T5.1: Implement PostgresProfileRepo with DictionaryRepository composition (AC-006)
type PostgresProfileRepo struct {
	pool     *pgxpool.Pool
	dictRepo dictionary.DictionaryRepository
}

func NewPostgresProfileRepo(pool *pgxpool.Pool, dictRepo dictionary.DictionaryRepository) *PostgresProfileRepo {
	return &PostgresProfileRepo{pool: pool, dictRepo: dictRepo}
}

func (r *PostgresProfileRepo) Save(ctx context.Context, profile *entity.Profile) error {
	return nil
}

func (r *PostgresProfileRepo) FindByID(ctx context.Context, id value.ProfileID) (*entity.Profile, error) {
	return nil, nil
}

func (r *PostgresProfileRepo) FindBySlug(ctx context.Context, tenantID value.TenantID, slug value.ProfileSlug) (*entity.Profile, error) {
	return nil, nil
}

func (r *PostgresProfileRepo) ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Profile, error) {
	return nil, nil
}

func (r *PostgresProfileRepo) Delete(ctx context.Context, id value.ProfileID) error {
	return nil
}
