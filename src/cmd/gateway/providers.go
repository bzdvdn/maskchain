package main

import (
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

func initProviders(routingCfg *config.RoutingConfig, egressCfg *config.EgressConfig) (*providerDeps, error) {
	domainCfg := toDomainRoutingConfig(routingCfg)
	registry, err := routingSvc.NewProviderRegistry(domainCfg)
	if err != nil {
		return nil, err
	}
	selector := routingSvc.NewRouteSelector(registry)

	clients := make(map[string]ports.ProviderClient)
	if routingCfg != nil {
		for i := range routingCfg.Providers {
			pcfg := &routingCfg.Providers[i]
			client, err := provider.NewProviderClient(pcfg, egressCfg)
			if err != nil {
				return nil, err
			}
			clients[pcfg.Name] = client
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
