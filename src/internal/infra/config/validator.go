package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

// @sk-task 111-provider-auth-and-config#T2.1: Validate APIKeys required + auth_scheme enum (AC-005)
// @sk-task ollama-provider#T1.1: Relax api_keys validation for ollama (AC-001)
//
// validateProviderAuth validates provider authentication configuration.
// Required: api_keys for non-ollama providers; auth_scheme must be bearer, api-key, or basic.
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

func validateConfig(cfg *Config, v *viper.Viper) error {
	if err := validateProviderAuth(cfg); err != nil {
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
