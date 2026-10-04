package service

import (
	"errors"
	"math/rand/v2"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

var (
	ErrNoRoute           = errors.New("no route for model")
	ErrNoHealthyProvider = errors.New("no healthy provider for model")
)

// @sk-task 70-routing-engine#T2.2: Implement RouteSelector (AC-001, AC-003, AC-004, AC-005)
//
// RouteSelector represents a domain entity or configuration.
type RouteSelector struct {
	registry *ProviderRegistry
	intN     func(max int) int
}

func NewRouteSelector(registry *ProviderRegistry) *RouteSelector {
	return &RouteSelector{registry: registry, intN: rand.IntN}
}

// @sk-task model-aliases-weighted-lb#T2.1: injectable RNG for deterministic tests (AC-006)
//
// NewRouteSelectorWithRand builds a selector with an explicit intN (0 <= intN(n) < n)
// so weighted selection can be tested deterministically.
func NewRouteSelectorWithRand(registry *ProviderRegistry, intN func(max int) int) *RouteSelector {
	if intN == nil {
		intN = rand.IntN
	}
	return &RouteSelector{registry: registry, intN: intN}
}

// @sk-task routing-ia#T1.1: tenant route with global fallback (AC-001, AC-002)
// @sk-task model-aliases-weighted-lb#T2.1: alias resolution + weighted pick (AC-001..AC-008)
//
// Select resolves the tenant alias, resolves the provider chain, filters to the
// healthy providers of the minimum priority tier, and picks a primary weighted
// by provider weight. It returns the primary and a fallback chain ordered with
// the primary first.
func (s *RouteSelector) Select(model string, tenantID string) (*routing.Provider, []string, error) {
	resolved, _ := s.registry.ResolveAlias(tenantID, model)

	providers, ok := s.providersFor(resolved, tenantID)
	if !ok {
		return nil, nil, ErrNoRoute
	}

	healthy := make([]*routing.Provider, 0, len(providers))
	for _, name := range providers {
		p := s.registry.Get(name)
		if p != nil && p.HealthStatus() == routing.HealthHealthy {
			healthy = append(healthy, p)
		}
	}
	if len(healthy) == 0 {
		return nil, providers, ErrNoHealthyProvider
	}

	primary := s.pick(healthy)
	return primary, orderChain(primary.Name, providers), nil
}

// pick selects among the healthy providers: the minimum priority tier only,
// then a weighted-random choice within that tier.
func (s *RouteSelector) pick(healthy []*routing.Provider) *routing.Provider {
	minPriority := healthy[0].Priority
	for _, p := range healthy {
		if p.Priority < minPriority {
			minPriority = p.Priority
		}
	}
	tier := make([]*routing.Provider, 0, len(healthy))
	for _, p := range healthy {
		if p.Priority == minPriority {
			tier = append(tier, p)
		}
	}
	return chooseWeighted(tier, s.intN)
}

// effectiveWeight treats a missing weight (0) as 1 so unweighted providers still
// participate rather than being dropped.
func effectiveWeight(p *routing.Provider) int {
	if p.Weight > 0 {
		return p.Weight
	}
	return 1
}

// chooseWeighted returns a provider from the tier in proportion to its effective
// weight. When no provider in the tier is explicitly weighted, it returns the
// first provider so unweighted deployments keep their deterministic ordered
// behavior.
func chooseWeighted(tier []*routing.Provider, intN func(max int) int) *routing.Provider {
	if len(tier) == 0 {
		return nil
	}
	anyWeighted := false
	for _, p := range tier {
		if p.Weight > 0 {
			anyWeighted = true
			break
		}
	}
	if !anyWeighted {
		return tier[0]
	}

	total := 0
	for _, p := range tier {
		total += effectiveWeight(p)
	}
	draw := intN(total)
	acc := 0
	for _, p := range tier {
		acc += effectiveWeight(p)
		if draw < acc {
			return p
		}
	}
	return tier[len(tier)-1]
}

// orderChain returns the provider names with primary first, then the rest in
// their declared order, forming the fallback chain.
func orderChain(primary string, names []string) []string {
	out := make([]string, 0, len(names))
	out = append(out, primary)
	for _, n := range names {
		if n != primary {
			out = append(out, n)
		}
	}
	return out
}

func (s *RouteSelector) GetProviderList(model string, tenantID string) ([]string, error) {
	providers, ok := s.providersFor(model, tenantID)
	if !ok {
		return nil, ErrNoRoute
	}
	return providers, nil
}

// @sk-task provider-model-registry#T2.1: wildcard fallback with precedence (AC-003, AC-004)
//
// providersFor resolves the provider chain for a model in precedence order:
// exact tenant route, wildcard tenant route, exact global route, wildcard
// global route. An empty tenant is normalized to "default" so existing
// deployments are unchanged.
func (s *RouteSelector) providersFor(model, tenantID string) ([]string, bool) {
	if tenantID == "" {
		tenantID = "default"
	}
	if providers, ok := s.lookup(tenantID, model); ok {
		return providers, true
	}
	if providers, ok := s.lookupWildcard(tenantID, model); ok {
		return providers, true
	}
	if tenantID != routing.GlobalTenant {
		if providers, ok := s.lookup(routing.GlobalTenant, model); ok {
			return providers, true
		}
		if providers, ok := s.lookupWildcard(routing.GlobalTenant, model); ok {
			return providers, true
		}
	}
	return nil, false
}

func (s *RouteSelector) lookup(tenantID, model string) ([]string, bool) {
	for _, rule := range s.registry.Rules() {
		if rule.TenantID != tenantID {
			continue
		}
		for _, route := range rule.Routes {
			if route.Model == model {
				return route.Providers, true
			}
		}
	}
	return nil, false
}

// lookupWildcard resolves the best wildcard route for a tenant: the longest
// literal prefix that matches the model wins; any other wildcard acts as a
// catch-all fallback. Among equal scores the first declared route wins.
func (s *RouteSelector) lookupWildcard(tenantID, model string) ([]string, bool) {
	var best []string
	bestScore := -1
	for _, rule := range s.registry.Rules() {
		if rule.TenantID != tenantID {
			continue
		}
		for _, route := range rule.Routes {
			if !routing.IsWildcardModel(route.Model) {
				continue
			}
			score := 0 // catch-all
			if prefix := routing.WildcardPrefix(route.Model); prefix != "" {
				if model == prefix || strings.HasPrefix(model, prefix+"/") {
					score = len(prefix) + 1
				}
			}
			if score > bestScore {
				best = route.Providers
				bestScore = score
			}
		}
	}
	return best, bestScore >= 0
}
