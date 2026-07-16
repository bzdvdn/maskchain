package entity

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task tenant-profile-sync#T1.1: Implement Tenant entity (AC-001, AC-002)
// @sk-task 13-shield-middleware-wiring#T1.1: Add PIIConfig for per-tenant PII rules (AC-001)
// @sk-task 13-shield-middleware-wiring#T2.2: Add mapstructure tags for YAML/viper deserialization
type PIIConfig struct {
	Enabled       bool      `json:"enabled" mapstructure:"enabled"`
	DefaultAction string    `json:"default_action" mapstructure:"default_action"`
	Rules         []PIARule `json:"rules" mapstructure:"rules"`
}

type PIARule struct {
	Label   string `json:"label" mapstructure:"label"`
	Type    string `json:"type" mapstructure:"type"`
	Pattern string `json:"pattern" mapstructure:"pattern"`
	Action  string `json:"action" mapstructure:"action"`
}

type Tenant struct {
	slug         value.TenantSlug
	name         string
	authHeader   string
	apiKeys      []string
	dictionaries []*dictionary.Dictionary
	piiConfig    PIIConfig
	createdAt    time.Time
	updatedAt    time.Time
}

type TenantOption func(*Tenant)

func WithTenantDictionaries(dicts []*dictionary.Dictionary) TenantOption {
	return func(t *Tenant) { t.dictionaries = dicts }
}

func WithTenantPIIConfig(cfg PIIConfig) TenantOption {
	return func(t *Tenant) { t.piiConfig = cfg }
}

func NewTenant(slug value.TenantSlug, name string, authHeader string, apiKeys []string, opts ...TenantOption) *Tenant {
	t := &Tenant{
		slug:       slug,
		name:       name,
		authHeader: authHeader,
		apiKeys:    apiKeys,
		createdAt:  time.Now().UTC(),
		updatedAt:  time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Tenant) Slug() value.TenantSlug                    { return t.slug }
func (t *Tenant) Name() string                              { return t.name }
func (t *Tenant) AuthHeader() string                        { return t.authHeader }
func (t *Tenant) APIKeys() []string                         { return t.apiKeys }
func (t *Tenant) Dictionaries() []*dictionary.Dictionary    { return t.dictionaries }
func (t *Tenant) SetDictionaries(dicts []*dictionary.Dictionary) { t.dictionaries = dicts }
func (t *Tenant) PIIConfig() PIIConfig                      { return t.piiConfig }
func (t *Tenant) CreatedAt() time.Time                      { return t.createdAt }
func (t *Tenant) UpdatedAt() time.Time                      { return t.updatedAt }
