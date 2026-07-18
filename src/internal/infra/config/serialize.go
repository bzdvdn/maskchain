package config

import (
	"log/slog"

	"go.uber.org/zap/zapcore"
)

func (c *Config) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("log_level", c.Log.Level)
	if c.Server != nil {
		enc.AddInt("port", c.Server.Port)
		enc.AddInt("shutdown_timeout", c.Server.ShutdownTimeout)
		enc.AddString("tenant_reload_interval", c.Server.TenantReloadInterval.String())
	}
	if c.Valkey != nil && c.Valkey.Addr != "" {
		enc.AddString("valkey_addr", c.Valkey.Addr)
		enc.AddString("valkey_password", "****")
		enc.AddInt("valkey_ttl_sec", c.Valkey.TTLSec)
	}
	if c.Routing != nil {
		_ = enc.AddArray("providers", zapcore.ArrayMarshalerFunc(func(aenc zapcore.ArrayEncoder) error {
			for _, p := range c.Routing.Providers {
				_ = aenc.AppendObject(providerLogEntryFromConfig(p))
			}
			return nil
		}))
	}
	if c.OTel != nil {
		enc.AddString("otel_endpoint", c.OTel.Endpoint)
	}
	if c.Tenants != nil {
		_ = enc.AddArray("tenants", zapcore.ArrayMarshalerFunc(func(aenc zapcore.ArrayEncoder) error {
			for slug := range c.Tenants {
				aenc.AppendString(slug)
			}
			return nil
		}))
	}
	return nil
}

type providerLogEntry struct {
	Name       string
	BaseURL    string
	APIType    string
	AuthScheme string
	AuthPrefix string
}

func (p providerLogEntry) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", p.Name)
	enc.AddString("base_url", p.BaseURL)
	enc.AddString("api_type", p.APIType)
	enc.AddString("api_keys", "****")
	enc.AddString("auth_scheme", p.AuthScheme)
	enc.AddString("auth_prefix", p.AuthPrefix)
	return nil
}

func providerLogEntryFromConfig(p ProviderConfig) providerLogEntry {
	return providerLogEntry{
		Name:       p.Name,
		BaseURL:    p.BaseURL,
		APIType:    p.APIType,
		AuthScheme: p.AuthScheme,
		AuthPrefix: p.AuthPrefix,
	}
}

// @sk-task 111-provider-auth-and-config#T2.2: Mask APIKeys via slog.LogValuer (AC-006)
func (p ProviderConfig) LogValue() slog.Value {
	masked := make([]string, len(p.APIKeys))
	for i := range p.APIKeys {
		masked[i] = "****"
	}
	return slog.GroupValue(
		slog.String("name", p.Name),
		slog.String("base_url", p.BaseURL),
		slog.String("health_endpoint", p.HealthEndpoint),
		slog.String("timeout", p.Timeout),
		slog.Int("priority", p.Priority),
		slog.String("api_type", p.APIType),
		slog.Any("api_keys", masked),
		slog.String("auth_scheme", p.AuthScheme),
		slog.String("auth_header", p.AuthHeader),
		slog.String("auth_prefix", p.AuthPrefix),
		slog.Any("additional_headers", p.AdditionalHeaders),
	)
}
