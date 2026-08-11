package bootstrap

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// LoadVirtualKeys returns a Postgres virtual key repository if a database pool
// is available, otherwise nil.
func NewVirtualKeyRepo(pgPool *pgxpool.Pool) *postgres.PostgresVirtualKeyRepository {
	if pgPool == nil {
		return nil
	}
	return postgres.NewPostgresVirtualKeyRepository(pgPool)
}

// BackfillVirtualKeys creates virtual keys for raw tenant api keys that came
// from the yaml config so existing clients keep working after the universal
// keys rollout. It is idempotent.
func BackfillVirtualKeys(ctx context.Context, cfg *config.Config, pgPool *pgxpool.Pool, logger *slog.Logger) int {
	repo := NewVirtualKeyRepo(pgPool)
	if repo == nil {
		return 0
	}
	legacy := make(map[string][]string, len(cfg.Tenants))
	for slug, tc := range cfg.Tenants {
		if len(tc.APIKeys) == 0 {
			continue
		}
		legacy[slug] = tc.APIKeys
	}
	if len(legacy) == 0 {
		return 0
	}
	created, err := repo.BackfillFromLegacy(ctx, legacy)
	if err != nil {
		logger.Error("failed to backfill virtual keys", slog.String("error", err.Error()))
		return 0
	}
	if created > 0 {
		logger.Info("backfilled legacy tenant keys into virtual keys", slog.Int("created", created))
	}
	return created
}
