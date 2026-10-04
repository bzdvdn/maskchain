package bootstrap

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	analyticsrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/analytics"
	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// yamlProvidersToRegistry converts yaml routing providers into registry provider
// configs carrying "yaml" provenance so SeedFromYAML can import them.
func yamlProvidersToRegistry(cfg *config.RoutingConfig) []routingDomain.ProviderConfig {
	if cfg == nil {
		return []routingDomain.ProviderConfig{}
	}
	out := make([]routingDomain.ProviderConfig, len(cfg.Providers))
	for i, p := range cfg.Providers {
		out[i] = routingDomain.ProviderConfig{
			Name:               p.Name,
			BaseURL:            p.BaseURL,
			HealthEndpoint:     p.HealthEndpoint,
			Timeout:            p.Timeout,
			Priority:           p.Priority,
			APIType:            p.APIType,
			APIKeys:            p.APIKeys,
			AuthScheme:         p.AuthScheme,
			AuthHeader:         p.AuthHeader,
			AuthPrefix:         p.AuthPrefix,
			AdditionalHeaders:  p.AdditionalHeaders,
			ProxyURL:           p.ProxyURL,
			Models:             p.Models,
			Weight:             p.Weight,
			AWSRegion:          p.AWSRegion,
			AWSAccessKeyID:     p.AWSAccessKeyID,
			AWSSecretAccessKey: p.AWSSecretAccessKey,
			Source:             "yaml",
		}
	}
	return out
}

// yamlRulesToRegistry converts yaml routing rules into registry rules carrying
// yaml provenance.
func yamlRulesToRegistry(cfg *config.RoutingConfig) []routingDomain.RuleConfig {
	if cfg == nil {
		return []routingDomain.RuleConfig{}
	}
	out := make([]routingDomain.RuleConfig, len(cfg.Rules))
	for i, r := range cfg.Rules {
		out[i] = routingDomain.RuleConfig{
			Tenant: r.Tenant,
			Source: "yaml",
			Routes: make([]routingDomain.RouteConfig, len(r.Routes)),
		}
		for j, rt := range r.Routes {
			out[i].Routes[j] = routingDomain.RouteConfig{Model: rt.Model, Providers: rt.Providers}
		}
	}
	return out
}

// yamlCostRatesToDomain converts yaml analytics cost rates into domain cost
// rates with a USD default currency and yaml provenance.
func yamlCostRatesToDomain(cfg *config.Config) []*analytics.CostRate {
	if cfg == nil || cfg.Analytics == nil {
		return []*analytics.CostRate{}
	}
	out := make([]*analytics.CostRate, 0, len(cfg.Analytics.CostRates))
	for _, r := range cfg.Analytics.CostRates {
		rate, err := analytics.NewCostRateWithCurrency(r.Model, r.InputPricePer1K, r.OutputPricePer1K, analytics.DefaultCurrency)
		if err != nil {
			continue
		}
		rate.Source = "yaml"
		out = append(out, rate)
	}
	return out
}

// yamlAliasesToRegistry converts yaml model aliases into registry aliases
// carrying yaml provenance so they can be seeded.
func yamlAliasesToRegistry(cfg *config.RoutingConfig) []routingDomain.AliasConfig {
	if cfg == nil {
		return []routingDomain.AliasConfig{}
	}
	out := make([]routingDomain.AliasConfig, 0, len(cfg.Aliases))
	for _, a := range cfg.Aliases {
		out = append(out, routingDomain.AliasConfig{
			Tenant: a.Tenant,
			Alias:  a.Alias,
			Target: a.Target,
			Source: "yaml",
		})
	}
	return out
}

// @sk-task model-aliases-weighted-lb#T1.5: carry aliases into the registry (AC-009)
//
// LoadRoutingFromDB seeds the registry from yaml when the tables are empty and
// returns the effective registry contents. When no database pool is available
// it falls back to the yaml configuration only.
func LoadRoutingFromDB(ctx context.Context, yamlCfg *config.RoutingConfig, pgPool *pgxpool.Pool, logger *slog.Logger) ([]routingDomain.ProviderConfig, []routingDomain.RuleConfig, []routingDomain.AliasConfig) {
	yamlProviders := yamlProvidersToRegistry(yamlCfg)
	yamlRules := yamlRulesToRegistry(yamlCfg)
	yamlAliases := yamlAliasesToRegistry(yamlCfg)
	if pgPool == nil {
		return yamlProviders, yamlRules, yamlAliases
	}
	repo := postgres.NewPostgresRegistryRepository(pgPool)
	if _, err := repo.SeedFromYAML(ctx, yamlProviders, yamlRules); err != nil {
		logger.Error("failed to seed routing registry", slog.String("error", err.Error()))
	}
	if _, err := repo.SeedAliasesFromYAML(ctx, yamlAliases); err != nil {
		logger.Error("failed to seed routing aliases", slog.String("error", err.Error()))
	}
	providers, err := repo.ListProviders(ctx)
	if err != nil {
		logger.Error("failed to list routing providers", slog.String("error", err.Error()))
		return yamlProviders, yamlRules, yamlAliases
	}
	rules, err := repo.ListRules(ctx)
	if err != nil {
		logger.Error("failed to list routing rules", slog.String("error", err.Error()))
		return yamlProviders, yamlRules, yamlAliases
	}
	aliases, err := repo.ListAliases(ctx)
	if err != nil {
		logger.Error("failed to list routing aliases", slog.String("error", err.Error()))
		return providers, rules, yamlAliases
	}
	return providers, rules, aliases
}

// LoadCostRatesFromDB seeds cost rates from yaml when the table is empty and
// returns the effective cost rates. When no database pool is available it
// falls back to the yaml configuration only.
func LoadCostRatesFromDB(ctx context.Context, cfg *config.Config, pgPool *pgxpool.Pool, logger *slog.Logger) []*analytics.CostRate {
	yamlRates := yamlCostRatesToDomain(cfg)
	if pgPool == nil {
		return yamlRates
	}
	store := analyticsrepo.NewPostgresCostRateStore(pgPool)
	if _, err := store.SeedFromYAML(ctx, yamlRates); err != nil {
		logger.Error("failed to seed cost rates", slog.String("error", err.Error()))
	}
	rates, err := store.List(ctx)
	if err != nil {
		logger.Error("failed to list cost rates", slog.String("error", err.Error()))
		return yamlRates
	}
	return rates
}
