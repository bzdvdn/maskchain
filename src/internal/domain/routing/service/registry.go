package service

import (
	"fmt"
	"sync/atomic"

	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-task 70-routing-engine#T2.1: Implement ProviderRegistry (AC-001)
type ProviderRegistry struct {
	providers atomic.Pointer[map[string]*routing.Provider]
	rules     atomic.Pointer[[]routing.RoutingRule]
}

func NewProviderRegistry(cfg *routing.RoutingConfig) (*ProviderRegistry, error) {
	reg := &ProviderRegistry{}
	if cfg == nil {
		reg.providers.Store(&map[string]*routing.Provider{})
		reg.rules.Store(&[]routing.RoutingRule{})
		return reg, nil
	}
	providers := make(map[string]*routing.Provider)
	for _, p := range cfg.Providers {
		if p.Name == "" {
			return nil, fmt.Errorf("provider name is required")
		}
		prov := routing.NewProvider(p.Name, p.BaseURL, p.HealthEndpoint, p.Timeout, p.Priority)
		prov.SetHealthStatus(routing.HealthHealthy)
		providers[p.Name] = prov
	}
	rules := make([]routing.RoutingRule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		var routes []routing.Route
		for _, rt := range r.Routes {
			routes = append(routes, routing.NewRoute(rt.Model, rt.Providers))
		}
		tenantID := r.Tenant
		if tenantID == "" {
			tenantID = "default"
		}
		rules = append(rules, routing.NewRoutingRule(tenantID, routes))
	}
	reg.providers.Store(&providers)
	reg.rules.Store(&rules)
	return reg, nil
}

// @sk-task config-hot-reload#T1.3: ProviderRegistry.UpdateConfig with atomic.Pointer (AC-001, AC-005)
func (r *ProviderRegistry) UpdateConfig(cfg *routing.RoutingConfig) error {
	newReg, err := NewProviderRegistry(cfg)
	if err != nil {
		return err
	}
	r.providers.Store(newReg.providers.Load())
	r.rules.Store(newReg.rules.Load())
	return nil
}

func (r *ProviderRegistry) Get(name string) *routing.Provider {
	m := r.providers.Load()
	if m == nil {
		return nil
	}
	return (*m)[name]
}

func (r *ProviderRegistry) List() []*routing.Provider {
	m := r.providers.Load()
	if m == nil {
		return nil
	}
	all := make([]*routing.Provider, 0, len(*m))
	for _, p := range *m {
		all = append(all, p)
	}
	return all
}

func (r *ProviderRegistry) Rules() []routing.RoutingRule {
	rl := r.rules.Load()
	if rl == nil {
		return nil
	}
	return *rl
}
