package bootstrap

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/budget"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// NewBudgetRepo returns a Postgres budget repository if a database pool is
// available, otherwise nil.
func NewBudgetRepo(pgPool *pgxpool.Pool) *postgres.PostgresBudgetRepository {
	if pgPool == nil {
		return nil
	}
	return postgres.NewPostgresBudgetRepository(pgPool)
}

// NewBudgetCounter returns a Valkey spend counter if a Valkey client is
// available, otherwise nil.
func NewBudgetCounter(vkClient valkey.Client) *budgetrepo.ValkeySpendCounter {
	if vkClient == nil {
		return nil
	}
	return budgetrepo.NewValkeySpendCounter(vkClient)
}

// BudgetAggregationInterval returns the configured aggregation interval or a
// sensible default.
func BudgetAggregationInterval(cfg *config.Config) string {
	if cfg.Budgets != nil && cfg.Budgets.AggregationInterval != "" {
		return cfg.Budgets.AggregationInterval
	}
	return "5m"
}
