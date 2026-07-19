package config

import (
	"log/slog"
)

// @sk-task 111-provider-auth-and-config#T2.2: Mask APIKeys via slog.LogValuer (AC-006)
// @sk-task provider-adapters-expansion#T1.1: Mask AWS secret access key in logs (AC-004)
func (p ProviderConfig) LogValue() slog.Value {
	masked := make([]string, len(p.APIKeys))
	for i := range p.APIKeys {
		masked[i] = "****"
	}
	attrs := []slog.Attr{
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
	}
	if p.AWSRegion != "" {
		attrs = append(attrs, slog.String("aws_region", p.AWSRegion))
	}
	if p.AWSAccessKeyID != "" {
		attrs = append(attrs, slog.String("aws_access_key_id", p.AWSAccessKeyID))
	}
	if p.AWSSecretAccessKey != "" {
		attrs = append(attrs, slog.String("aws_secret_access_key", "****"))
	}
	return slog.GroupValue(attrs...)
}
