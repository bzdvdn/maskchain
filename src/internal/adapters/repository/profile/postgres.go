package profilerepo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 24-shield-dictionaries#T5.1: Implement PostgresProfileRepo with DictionaryRepository composition (AC-006)
type PostgresProfileRepo struct {
	pool          *pgxpool.Pool
	dictRepo      dictionary.DictionaryRepository
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
