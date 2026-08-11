package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/cmd/internal/bootstrap"
	"github.com/bzdvdn/maskchain/src/internal/adapters/provider"
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

type providerDeps struct {
	registry        *routingSvc.ProviderRegistry
	selector        *routingSvc.RouteSelector
	fallbackHandler *routingSvc.FallbackHandler
	clients         map[string]ports.ProviderClient
}

// @sk-task 150-admin-routing-crud#T5.1: Gateway resolves routing from the DB registry with yaml fallback
func initProviders(routingCfg *config.RoutingConfig, egressCfg *config.EgressConfig, pgPool *pgxpool.Pool, logger *slog.Logger) (*providerDeps, error) {
	providers, rules := bootstrap.LoadRoutingFromDB(context.Background(), routingCfg, pgPool, logger)
	domainCfg := &routingDomain.RoutingConfig{Providers: providers, Rules: rules}
	registry, err := routingSvc.NewProviderRegistry(domainCfg)
	if err != nil {
		return nil, err
	}
	selector := routingSvc.NewRouteSelector(registry)

	clients := make(map[string]ports.ProviderClient)
	if egressCfg != nil {
		for _, p := range fromDomainProviders(providers) {
			client, err := provider.NewProviderClient(&p, egressCfg)
			if err != nil {
				logger.Error("failed to create provider client", slog.String("provider", p.Name), slog.String("error", err.Error()))
				continue
			}
			clients[p.Name] = client
		}
	}
	fallbackHandler := routingSvc.NewFallbackHandler(clients)

	return &providerDeps{
		registry:        registry,
		selector:        selector,
		fallbackHandler: fallbackHandler,
		clients:         clients,
	}, nil
}

// toDomainRoutingConfig converts yaml routing into a domain routing config.
func toDomainRoutingConfig(cfg *config.RoutingConfig) *routingDomain.RoutingConfig {
	if cfg == nil {
		return nil
	}
	domainCfg := &routingDomain.RoutingConfig{
		Providers: make([]routingDomain.ProviderConfig, len(cfg.Providers)),
		Rules:     make([]routingDomain.RuleConfig, 0, len(cfg.Rules)),
	}
	for i, p := range cfg.Providers {
		domainCfg.Providers[i] = routingDomain.ProviderConfig{
			Name:           p.Name,
			BaseURL:        p.BaseURL,
			HealthEndpoint: p.HealthEndpoint,
			Timeout:        p.Timeout,
			Priority:       p.Priority,
			APIType:        p.APIType,
			APIKeys:        p.APIKeys,
			AuthScheme:     p.AuthScheme,
			AuthHeader:     p.AuthHeader,
			AuthPrefix:     p.AuthPrefix,
		}
	}
	for _, r := range cfg.Rules {
		routes := make([]routingDomain.RouteConfig, len(r.Routes))
		for j, rt := range r.Routes {
			routes[j] = routingDomain.RouteConfig{
				Model:     rt.Model,
				Providers: rt.Providers,
			}
		}
		domainCfg.Rules = append(domainCfg.Rules, routingDomain.RuleConfig{
			Tenant: r.Tenant,
			Routes: routes,
		})
	}
	return domainCfg
}

// fromDomainProviders converts registry provider configs into config-level
// provider configs suitable for building provider clients.
func fromDomainProviders(providers []routingDomain.ProviderConfig) []config.ProviderConfig {
	out := make([]config.ProviderConfig, 0, len(providers))
	for _, p := range providers {
		out = append(out, config.ProviderConfig{
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
			AWSRegion:          p.AWSRegion,
			AWSAccessKeyID:     p.AWSAccessKeyID,
			AWSSecretAccessKey: p.AWSSecretAccessKey,
		})
	}
	return out
}
