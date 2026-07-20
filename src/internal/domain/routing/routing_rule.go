package routing

// @sk-task 70-routing-engine#T1.1: Create RoutingRule entity (AC-005)
//
// RoutingRule represents a domain entity or configuration.
type RoutingRule struct {
	TenantID string
	Routes   []Route
}

func NewRoutingRule(tenantID string, routes []Route) RoutingRule {
	return RoutingRule{TenantID: tenantID, Routes: routes}
}
