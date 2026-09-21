package service

import (
	"errors"

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
}

func NewRouteSelector(registry *ProviderRegistry) *RouteSelector {
	return &RouteSelector{registry: registry}
}

// @sk-task routing-ia#T1.1: tenant route with global fallback (AC-001, AC-002)
func (s *RouteSelector) Select(model string, tenantID string) (*routing.Provider, []string, error) {
	providers, ok := s.providersFor(model, tenantID)
	if !ok {
		return nil, nil, ErrNoRoute
	}
	for _, name := range providers {
		p := s.registry.Get(name)
		if p == nil {
			continue
		}
		if p.HealthStatus() == routing.HealthHealthy {
			return p, providers, nil
		}
	}
	return nil, providers, ErrNoHealthyProvider
}

func (s *RouteSelector) GetProviderList(model string, tenantID string) ([]string, error) {
	providers, ok := s.providersFor(model, tenantID)
	if !ok {
		return nil, ErrNoRoute
	}
	return providers, nil
}

// providersFor resolves the provider chain for a model: the tenant-specific
// route wins, otherwise the global default (GlobalTenant) is used. An empty
// tenant is normalized to "default" so existing deployments are unchanged.
func (s *RouteSelector) providersFor(model, tenantID string) ([]string, bool) {
	if tenantID == "" {
		tenantID = "default"
	}
	if providers, ok := s.lookup(tenantID, model); ok {
		return providers, true
	}
	if tenantID != routing.GlobalTenant {
		if providers, ok := s.lookup(routing.GlobalTenant, model); ok {
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
