package routing

import "context"

// @sk-task 150-admin-routing-crud#T1.1: RegistryRepository for routing config CRUD (AC-001)
//
// RegistryRepository persists routing providers and model routes.
type RegistryRepository interface {
	// ListProviders returns all providers, merged yaml-source and ui-source.
	ListProviders(ctx context.Context) ([]ProviderConfig, error)
	// ListRules returns all routing rules with their model routes.
	ListRules(ctx context.Context) ([]RuleConfig, error)
	// UpsertProvider creates or updates a provider. A ui upsert detaches it
	// from the read-only yaml default (source becomes "ui").
	UpsertProvider(ctx context.Context, p ProviderConfig) error
	// DeleteProvider removes a provider and clears references from routes.
	DeleteProvider(ctx context.Context, name string) error
	// UpsertRoute creates or updates a single model route inside a rule.
	UpsertRoute(ctx context.Context, tenant, model string, providers []string) error
	// DeleteRoute removes a model route.
	DeleteRoute(ctx context.Context, tenant, model string) error
	// SeedFromYAML inserts yaml defaults only when the tables are empty.
	// Returns true if seeding was performed.
	SeedFromYAML(ctx context.Context, providers []ProviderConfig, rules []RuleConfig) (bool, error)
}
