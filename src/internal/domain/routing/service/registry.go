package service

import (
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 70-routing-engine#T2.1: Implement ProviderRegistry (AC-001)
type ProviderRegistry struct {
	providers map[string]*routing.Provider
	rules     []routing.RoutingRule
}

func NewProviderRegistry(cfg *config.RoutingConfig) (*ProviderRegistry, error) {
	reg := &ProviderRegistry{
		providers: make(map[string]*routing.Provider),
	}
	if cfg == nil {
		return reg, nil
	}
	for _, p := range cfg.Providers {
		if p.Name == "" {
			return nil, fmt.Errorf("provider name is required")
		}
		prov := routing.NewProvider(p.Name, p.BaseURL, p.HealthEndpoint, p.Timeout, p.Priority)
		prov.SetHealthStatus(routing.HealthHealthy)
		reg.providers[p.Name] = prov
	}
	for _, r := range cfg.Rules {
		var routes []routing.Route
		for _, rt := range r.Routes {
			routes = append(routes, routing.NewRoute(rt.Model, rt.Providers))
		}
		tenantID := r.Tenant
		if tenantID == "" {
			tenantID = "default"
		}
		reg.rules = append(reg.rules, routing.NewRoutingRule(tenantID, routes))
	}
	return reg, nil
}

func (r *ProviderRegistry) Get(name string) *routing.Provider {
	return r.providers[name]
}

func (r *ProviderRegistry) List() []*routing.Provider {
	all := make([]*routing.Provider, 0, len(r.providers))
	for _, p := range r.providers {
		all = append(all, p)
	}
	return all
}
