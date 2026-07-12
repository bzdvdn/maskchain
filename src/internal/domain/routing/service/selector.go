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
type RouteSelector struct {
	registry *ProviderRegistry
}

func NewRouteSelector(registry *ProviderRegistry) *RouteSelector {
	return &RouteSelector{registry: registry}
}

func (s *RouteSelector) Select(model string, tenantID string) (*routing.Provider, []string, error) {
	if tenantID == "" {
		tenantID = "default"
	}
	for _, rule := range s.registry.rules {
		if rule.TenantID != tenantID {
			continue
		}
		for _, route := range rule.Routes {
			if route.Model != model {
				continue
			}
			for _, name := range route.Providers {
				p := s.registry.Get(name)
				if p == nil {
					continue
				}
				if p.HealthStatus() == routing.HealthHealthy {
					return p, route.Providers, nil
				}
			}
			return nil, route.Providers, ErrNoHealthyProvider
		}
	}
	return nil, nil, ErrNoRoute
}

func (s *RouteSelector) GetProviderList(model string, tenantID string) ([]string, error) {
	if tenantID == "" {
		tenantID = "default"
	}
	for _, rule := range s.registry.rules {
		if rule.TenantID != tenantID {
			continue
		}
		for _, route := range rule.Routes {
			if route.Model == model {
				return route.Providers, nil
			}
		}
	}
	return nil, ErrNoRoute
}
