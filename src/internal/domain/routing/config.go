package routing

import "errors"

// ErrNotFound is returned when a routing entity does not exist.
var ErrNotFound = errors.New("routing entity not found")

// ErrModelsUnsupported is returned when a provider type has no models API to
// discover models from.
var ErrModelsUnsupported = errors.New("provider does not expose a models API")

// @sk-task routing-ia#T1.1: reserved global tenant marker (AC-001, AC-002)
//
// GlobalTenant is a reserved tenant value. A route stored under it is the
// default for every tenant that has no explicit route for that model. It is not
// a real tenant slug and must never be offered as one.
const GlobalTenant = "*"

// @sk-task 70-routing-engine#T1.1: Domain-level provider config (DDD boundary)
//
// ProviderConfig represents a domain entity or configuration.
type ProviderConfig struct {
	Name           string
	BaseURL        string
	HealthEndpoint string
	Timeout        string
	Priority       int
	// Weight is the relative share used when several healthy providers share
	// the minimum priority tier. 0 means unweighted (effective weight 1).
	Weight            int
	APIType           string
	APIKeys           []string
	AuthScheme        string
	AuthHeader        string
	AuthPrefix        string
	AdditionalHeaders map[string]string
	ProxyURL          string
	// Models declares the models the provider serves; seeded as global routes.
	Models             []string
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

// @sk-task model-aliases-weighted-lb#T1.1: tenant-scoped model alias (AC-001, AC-009)
//
// AliasConfig maps a requested model name to the model that should actually be
// routed for a tenant (or the reserved global tenant "*").
type AliasConfig struct {
	Tenant string
	Alias  string
	Target string
	// Source records provenance: "yaml" or "ui".
	Source string
}

type RoutingConfig struct {
	Providers []ProviderConfig
	Rules     []RuleConfig
	Aliases   []AliasConfig
}
