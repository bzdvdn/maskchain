package config

import (
	"fmt"
	"reflect"

	"github.com/spf13/viper"
)

// @sk-task config-hot-reload#T1.2: DiffSections for runtime sections (AC-006)
func DiffSections(old, new *Config) map[string]bool {
	changed := make(map[string]bool)
	if old == nil || new == nil {
		return changed
	}
	if !reflect.DeepEqual(old.Routing, new.Routing) {
		changed["routing"] = true
	}
	if !reflect.DeepEqual(old.Tenants, new.Tenants) {
		changed["tenants"] = true
	}
	if !reflect.DeepEqual(old.Shield, new.Shield) {
		changed["shield"] = true
	}
	if !reflect.DeepEqual(old.RateLimit, new.RateLimit) {
		changed["ratelimit"] = true
	}
	if !reflect.DeepEqual(old.Debug, new.Debug) {
		changed["debug"] = true
	}
	return changed
}

// deepMergeMaps recursively merges src into dst. For nested maps, it recurses.
// For all other types (including slices), src overwrites dst.
func deepMergeMaps(dst, src map[string]interface{}) {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		srcMap, srcIsMap := sv.(map[string]interface{})
		dstMap, dstIsMap := dv.(map[string]interface{})
		if srcIsMap && dstIsMap {
			deepMergeMaps(dstMap, srcMap)
		} else {
			dst[k] = sv
		}
	}
}

// @sk-task 111-provider-auth-and-config#T1.2: Fallback for old api_key + apply defaults (AC-003)
func normalizeProviderConfig(cfg *Config, v *viper.Viper) {
	if cfg.Routing == nil {
		return
	}
	for i := range cfg.Routing.Providers {
		p := &cfg.Routing.Providers[i]
		if len(p.APIKeys) == 0 {
			oldKey := v.GetString(fmt.Sprintf("routing.providers.%d.api_key", i))
			if oldKey != "" {
				p.APIKeys = []string{oldKey}
			}
		}
		if p.AuthScheme == "" {
			p.AuthScheme = "bearer"
		}
		if p.AuthHeader == "" {
			p.AuthHeader = "Authorization"
		}
		if p.AuthPrefix == "" {
			switch p.AuthScheme {
			case "bearer":
				p.AuthPrefix = "Bearer "
			case "basic":
				p.AuthPrefix = "Basic "
			default:
				p.AuthPrefix = ""
			}
		}
	}
}
