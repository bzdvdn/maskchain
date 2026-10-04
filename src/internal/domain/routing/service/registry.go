package service

import (
	"fmt"
	"sync/atomic"

	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-task 70-routing-engine#T2.1: Implement ProviderRegistry (AC-001)
//
// ProviderRegistry represents a domain entity or configuration.
type ProviderRegistry struct {
	providers atomic.Pointer[map[string]*routing.Provider]
	rules     atomic.Pointer[[]routing.RoutingRule]
	aliases   atomic.Pointer[map[string]map[string]string]
}

func NewProviderRegistry(cfg *routing.RoutingConfig) (*ProviderRegistry, error) {
	reg := &ProviderRegistry{}
	emptyAliases := map[string]map[string]string{}
	if cfg == nil {
		reg.providers.Store(&map[string]*routing.Provider{})
		reg.rules.Store(&[]routing.RoutingRule{})
		reg.aliases.Store(&emptyAliases)
		return reg, nil
	}
	providers := make(map[string]*routing.Provider)
	for _, p := range cfg.Providers {
		if p.Name == "" {
			return nil, fmt.Errorf("provider name is required")
		}
		prov := routing.NewProvider(p.Name, p.BaseURL, p.HealthEndpoint, p.Timeout, p.Priority)
		prov.Weight = p.Weight
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
	aliases := make(map[string]map[string]string)
	for _, a := range cfg.Aliases {
		if a.Tenant == "" || a.Alias == "" || a.Target == "" {
			continue
		}
		if aliases[a.Tenant] == nil {
			aliases[a.Tenant] = map[string]string{}
		}
		aliases[a.Tenant][a.Alias] = a.Target
	}
	reg.providers.Store(&providers)
	reg.rules.Store(&rules)
	reg.aliases.Store(&aliases)
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
	r.aliases.Store(newReg.aliases.Load())
	return nil
}

// @sk-task model-aliases-weighted-lb#T2.1: tenant alias lookup (AC-001, AC-002)
//
// ResolveAlias returns the routed model for a requested model and tenant,
// checking the tenant map then the global tenant map. An empty tenant is
// normalized to "default".
func (r *ProviderRegistry) ResolveAlias(tenantID, model string) (string, bool) {
	m := r.aliases.Load()
	if m == nil {
		return model, false
	}
	t := tenantID
	if t == "" {
		t = "default"
	}
	return routing.ResolveAlias(*m, t, model)
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
