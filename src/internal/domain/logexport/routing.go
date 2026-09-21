package logexport

import "sort"

// @sk-task log-export#T1.1: per-tenant sink routing (AC-001, AC-003)
//
// Resolver maps a tenant to the sink names it is allowed to export to. A tenant
// with no entry resolves to no sinks, so export is inert until an operator
// routes the tenant.
type Resolver struct {
	routes map[string][]string
}

// NewResolver builds a resolver from a tenant -> sink names map. Nil or empty
// input yields a resolver that returns no sinks for any tenant.
func NewResolver(routes map[string][]string) *Resolver {
	m := make(map[string][]string, len(routes))
	for tenant, sinks := range routes {
		if len(sinks) == 0 {
			continue
		}
		names := make([]string, len(sinks))
		copy(names, sinks)
		sort.Strings(names)
		m[tenant] = names
	}
	return &Resolver{routes: m}
}

// SinksFor returns the sink names enabled for a tenant, or nil when none are.
func (r *Resolver) SinksFor(tenant string) []string {
	if r == nil {
		return nil
	}
	names, ok := r.routes[tenant]
	if !ok {
		return nil
	}
	out := make([]string, len(names))
	copy(out, names)
	return out
}
