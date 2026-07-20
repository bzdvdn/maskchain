package routing

// @sk-task 70-routing-engine#T1.1: Domain-level provider config (DDD boundary)
//
// ProviderConfig represents a domain entity or configuration.
type ProviderConfig struct {
	Name           string
	BaseURL        string
	HealthEndpoint string
	Timeout        string
	Priority       int
}

type RouteConfig struct {
	Model     string
	Providers []string
}

type RuleConfig struct {
	Tenant string
	Routes []RouteConfig
}

type RoutingConfig struct {
	Providers []ProviderConfig
	Rules     []RuleConfig
}
