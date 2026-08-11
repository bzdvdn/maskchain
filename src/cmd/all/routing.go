package main

import (
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

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
