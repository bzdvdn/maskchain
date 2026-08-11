package routing

import "errors"

// ErrNotFound is returned when a routing entity does not exist.
var ErrNotFound = errors.New("routing entity not found")

// @sk-task 70-routing-engine#T1.1: Domain-level provider config (DDD boundary)
//
// ProviderConfig represents a domain entity or configuration.
type ProviderConfig struct {
	Name               string
	BaseURL            string
	HealthEndpoint     string
	Timeout            string
	Priority           int
	APIType            string
	APIKeys            []string
	AuthScheme         string
	AuthHeader         string
	AuthPrefix         string
	AdditionalHeaders  map[string]string
	ProxyURL           string
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	// Source records provenance: "yaml" (seeded at startup) or "ui" (edited via admin).
	Source string
}

type RouteConfig struct {
	Model     string
	Providers []string
}

type RuleConfig struct {
	Tenant string
	Routes []RouteConfig
	// Source records provenance: "yaml" or "ui".
	Source string
}

type RoutingConfig struct {
	Providers []ProviderConfig
	Rules     []RuleConfig
}
