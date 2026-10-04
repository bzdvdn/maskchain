package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/viper"

	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
)

// @sk-task 111-provider-auth-and-config#T2.1: Validate APIKeys required + auth_scheme enum (AC-005)
// @sk-task ollama-provider#T1.1: Relax api_keys validation for ollama (AC-001)
//
// validateProviderAuth validates provider authentication configuration.
// Required: api_keys for non-ollama providers; auth_scheme must be bearer, api-key, or basic.
// @sk-task semantic-cache-masked#T1.2: validate data.cache config (AC-004)
//
// validateDataCache validates semantic cache thresholds and embedding source.
func validateDataCache(cfg *Config) error {
	c := cfg.Data
	if c == nil || c.Cache == nil || !c.Cache.Enabled {
		return nil
	}
	cacheCfg := c.Cache
	if cacheCfg.TTLSec <= 0 {
		return fmt.Errorf("data.cache.ttl: must be > 0, got %d", cacheCfg.TTLSec)
	}
	if cacheCfg.SimilarityThreshold < 0 || cacheCfg.SimilarityThreshold >= 1 {
		return fmt.Errorf("data.cache.similarity_threshold: must be in [0,1), got %f", cacheCfg.SimilarityThreshold)
	}
	if cacheCfg.BudgetGuardPercent < 0 || cacheCfg.BudgetGuardPercent > 100 {
		return fmt.Errorf("data.cache.budget_guard_percent: must be in [0,100], got %f", cacheCfg.BudgetGuardPercent)
	}
	if cacheCfg.MaxEntryBytes <= 0 {
		return fmt.Errorf("data.cache.max_entry_bytes: must be > 0, got %d", cacheCfg.MaxEntryBytes)
	}
	if cacheCfg.Embedding == nil {
		return fmt.Errorf("data.cache.embedding: required")
	}
	switch cacheCfg.Embedding.Source {
	case "external":
		if cacheCfg.Embedding.ExternalURL == "" {
			return fmt.Errorf("data.cache.embedding.external_url: required when source=external")
		}
	case "self-contained":
	default:
		return fmt.Errorf("data.cache.embedding.source: must be external|self-contained, got %q", cacheCfg.Embedding.Source)
	}
	return nil
}

func validateProviderAuth(cfg *Config) error {
	if cfg.Routing == nil {
		return nil
	}
	for i, p := range cfg.Routing.Providers {
		if p.Name == "" {
			continue
		}
		if len(p.APIKeys) == 0 {
			if p.APIType == "ollama" {
				continue
			}
			return fmt.Errorf("routing.providers.%d.api_keys: required for provider %q", i, p.Name)
		}
		switch p.AuthScheme {
		case "bearer", "api-key", "basic":
		default:
			return fmt.Errorf("routing.providers.%d.auth_scheme: unsupported %q (must be bearer, api-key, or basic)", i, p.AuthScheme)
		}
		if p.AuthScheme != "bearer" && p.AuthPrefix == "" {
			p.AuthPrefix = ""
		}
	}
	return nil
}

// validateRoutingRoutes rejects malformed wildcard patterns and wildcard routes
// that reference an unknown provider.
func validateRoutingRoutes(cfg *Config) error {
	if cfg.Routing == nil {
		return nil
	}
	providers := make(map[string]struct{}, len(cfg.Routing.Providers))
	for _, p := range cfg.Routing.Providers {
		if p.Name != "" {
			providers[p.Name] = struct{}{}
		}
	}
	for i, rule := range cfg.Routing.Rules {
		for j, rt := range rule.Routes {
			if !strings.Contains(rt.Model, "*") {
				continue
			}
			if strings.Count(rt.Model, "*") != 1 || (rt.Model != "*" && !strings.HasSuffix(rt.Model, "/*")) {
				return fmt.Errorf("routing.rules.%d.routes.%d.model: malformed wildcard pattern %q", i, j, rt.Model)
			}
			for _, name := range rt.Providers {
				if _, ok := providers[name]; !ok {
					return fmt.Errorf("routing.rules.%d.routes.%d: wildcard %q references unknown provider %q", i, j, rt.Model, name)
				}
			}
		}
	}
	return nil
}

// @sk-task model-aliases-weighted-lb#T3.2: provider weight + alias validation (AC-010)
func validateRoutingWeightAndAliases(cfg *Config) error {
	if cfg.Routing == nil {
		return nil
	}
	for i, p := range cfg.Routing.Providers {
		if p.Weight < 0 {
			return fmt.Errorf("routing.providers.%d.weight: must not be negative, got %d", i, p.Weight)
		}
	}
	for i, a := range cfg.Routing.Aliases {
		if a.Alias == "" || a.Target == "" {
			return fmt.Errorf("routing.aliases.%d: alias and target are required", i)
		}
	}
	return nil
}

// validateCompliance validates the compliance preset configuration.
func validateCompliance(cfg *Config) error {
	c := cfg.Compliance
	if c == nil {
		return nil
	}
	if c.PresetDir == "" {
		return fmt.Errorf("compliance.preset_dir: required")
	}
	for i, key := range c.EnabledPacks {
		if key == "" {
			return fmt.Errorf("compliance.enabled_packs.%d: empty pack key", i)
		}
	}
	return nil
}

// validateDataRetention validates data.retention.mode against the enum.
func validateDataRetention(cfg *Config) error {
	d := cfg.Data
	if d == nil || d.Retention == nil {
		return nil
	}
	switch d.Retention.Mode {
	case "full", "meta", "none":
	default:
		return fmt.Errorf("data.retention.mode: must be full|meta|none, got %q", d.Retention.Mode)
	}
	return nil
}

// @sk-task 403-key-at-rest-encryption#T1.1: Validate at-rest keys key fail-closed (AC-008)
// @sk-task 403-key-at-rest-encryption#T4.5: Tenants now require DB-backed virtual-key auth (AC-007)
//
// validateAtRestKeysKey requires a valid 32-byte base64 key (MASKCHAIN_KEYS_KEY
// or crypto.keys_key) whenever database-backed routing or tenancy is enabled.
// A missing or malformed key fails closed so provider secrets are never
// persisted as plaintext.
func validateAtRestKeysKey(cfg *Config) error {
	if cfg.DB == nil || cfg.DB.DSN == "" {
		// Raw-key in-memory auth was removed (T4.3); tenancy is now served only
		// by DB-backed virtual keys, so configured tenants without a DB fail closed.
		if len(cfg.Tenants) > 0 {
			return fmt.Errorf("tenants: configured but database is disabled; virtual-key auth requires DB (set db.dsn)")
		}
		return nil
	}
	routingEnabled := cfg.Routing != nil && len(cfg.Routing.Providers) > 0
	tenancyEnabled := len(cfg.Tenants) > 0
	if !routingEnabled && !tenancyEnabled {
		return nil
	}

	key := os.Getenv(KeysKeyEnvVar)
	if key == "" && cfg.Crypto != nil {
		key = cfg.Crypto.KeysKey
	}
	if key == "" {
		return fmt.Errorf("%s: required when database-backed routing or tenancy is enabled (set %s or crypto.keys_key)", KeysKeyEnvVar, KeysKeyEnvVar)
	}
	if _, err := crypto.New(key); err != nil {
		return fmt.Errorf("%s: %w", KeysKeyEnvVar, err)
	}
	return nil
}

func validateConfig(cfg *Config, v *viper.Viper) error {
	if err := validateAtRestKeysKey(cfg); err != nil {
		return err
	}
	if err := validateDataCache(cfg); err != nil {
		return err
	}
	if err := validateDataRetention(cfg); err != nil {
		return err
	}
	if err := validateCompliance(cfg); err != nil {
		return err
	}
	if err := validateProviderAuth(cfg); err != nil {
		return err
	}
	if err := validateRoutingRoutes(cfg); err != nil {
		return err
	}
	if err := validateRoutingWeightAndAliases(cfg); err != nil {
		return err
	}
	val := reflect.ValueOf(cfg).Elem()
	t := val.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		sub := val.Field(i)
		if sub.Kind() == reflect.Ptr && !sub.IsNil() {
			prefix := field.Tag.Get("mapstructure")
			if prefix == "" {
				prefix = strings.ToLower(field.Name)
			}
			if err := validateRequiredFields(sub.Elem(), v, prefix); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRequiredFields(val reflect.Value, v *viper.Viper, prefix string) error {
	t := val.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		validateTag := field.Tag.Get("validate")
		if !strings.Contains(validateTag, "required") {
			continue
		}
		mapKey := field.Tag.Get("mapstructure")
		if mapKey == "" {
			mapKey = strings.ToLower(field.Name)
		}
		fullKey := prefix + "." + mapKey
		if !v.IsSet(fullKey) {
			return fmt.Errorf("missing required field: %s", fullKey)
		}
	}
	return nil
}
