package routing

import "strings"

// @sk-task 70-routing-engine#T1.1: Create Route entity (AC-001)
//
// Route represents a domain entity or configuration.
type Route struct {
	Model     string
	Providers []string
}

func NewRoute(model string, providers []string) Route {
	return Route{Model: model, Providers: providers}
}

// @sk-task provider-model-registry#T2.1: wildcard route helpers (AC-003, AC-004)
//
// IsWildcardModel reports whether a route model is a wildcard pattern (contains
// "*"). Wildcard routes act as catch-alls for models without an exact route.
func IsWildcardModel(model string) bool {
	return strings.Contains(model, "*")
}

// WildcardPrefix returns the literal prefix before the first "*" in a wildcard
// pattern, without a trailing slash. "*" yields "", "groq/*" yields "groq".
func WildcardPrefix(model string) string {
	i := strings.Index(model, "*")
	if i < 0 {
		return ""
	}
	return strings.TrimSuffix(model[:i], "/")
}
