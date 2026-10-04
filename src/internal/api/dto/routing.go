package dto

import (
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-task 403-key-at-rest-encryption#T2.3: MaskSecret masks a secret for admin display (AC-003)
//
// MaskSecret returns the first 3 + "***" + last 3 characters when the secret is
// longer than 8, otherwise just "***". It is used to surface enough of a
// provider secret for identification without leaking the full value.
func MaskSecret(s string) string {
	if s == "" || len(s) <= 8 {
		return "***"
	}
	return s[:3] + "***" + s[len(s)-3:]
}

// WithMaskedSecrets returns copies of p's secret fields masked. Non-secret
// attributes are passed through unchanged so callers can echo the same DTO.
func WithMaskedSecrets(keys []string, awsAccess, awsSecret string) ([]string, string, string) {
	maskedKeys := make([]string, len(keys))
	for i, k := range keys {
		maskedKeys[i] = MaskSecret(k)
	}
	return maskedKeys, MaskSecret(awsAccess), MaskSecret(awsSecret)
}

// @sk-task 150-admin-routing-crud#T2.1: Routing registry DTOs (AC-001, AC-002)
//
// ProviderRequest represents a domain entity or configuration.
type ProviderRequest struct {
	Name           string `json:"name" binding:"required"`
	APIType        string `json:"api_type"`
	BaseURL        string `json:"base_url" binding:"required"`
	HealthEndpoint string `json:"health_endpoint"`
	Timeout        string `json:"timeout"`
	Priority       int    `json:"priority"`
	// @sk-task model-aliases-weighted-lb#T3.1: provider weight (AC-009)
	Weight             int               `json:"weight"`
	APIKeys            []string          `json:"api_keys"`
	AuthScheme         string            `json:"auth_scheme"`
	AuthHeader         string            `json:"auth_header"`
	AuthPrefix         string            `json:"auth_prefix"`
	AdditionalHeaders  map[string]string `json:"additional_headers"`
	ProxyURL           string            `json:"proxy_url"`
	AWSRegion          string            `json:"aws_region"`
	AWSAccessKeyID     string            `json:"aws_access_key_id"`
	AWSSecretAccessKey string            `json:"aws_secret_access_key"`
	// @sk-task routing-ia#T1.2: optional model ids attached on save (AC-004)
	Models []string `json:"models,omitempty"`
}

// @sk-task routing-ia#T1.2: model aggregate for the Models page (AC-003)
//
// ModelAggregate is one row of GET /api/v1/routing/models: the model's cost, its
// global default providers, and how many tenants override it.
type ModelAggregate struct {
	Model            string   `json:"model"`
	InputPricePer1K  float64  `json:"input_price_per_1k"`
	OutputPricePer1K float64  `json:"output_price_per_1k"`
	Currency         string   `json:"currency"`
	DefaultProviders []string `json:"default_providers"`
	OverrideCount    int      `json:"override_count"`
	Source           string   `json:"source"`
}

type ProviderResponse struct {
	Name               string            `json:"name"`
	APIType            string            `json:"api_type"`
	BaseURL            string            `json:"base_url"`
	HealthEndpoint     string            `json:"health_endpoint"`
	Timeout            string            `json:"timeout"`
	Priority           int               `json:"priority"`
	Weight             int               `json:"weight"`
	APIKeys            []string          `json:"api_keys"`
	AuthScheme         string            `json:"auth_scheme"`
	AuthHeader         string            `json:"auth_header"`
	AuthPrefix         string            `json:"auth_prefix"`
	AdditionalHeaders  map[string]string `json:"additional_headers,omitempty"`
	ProxyURL           string            `json:"proxy_url"`
	AWSRegion          string            `json:"aws_region"`
	AWSAccessKeyID     string            `json:"aws_access_key_id"`
	AWSSecretAccessKey string            `json:"aws_secret_access_key"`
	Source             string            `json:"source"`
	Status             string            `json:"status"`
	LatencyMs          int64             `json:"latency_ms,omitempty"`
	LastCheck          int64             `json:"last_check,omitempty"`
}

func ProviderToResponse(p routingDomain.ProviderConfig, status string, latency int64, lastCheck int64) ProviderResponse {
	maskedKeys, maskedAWSAccess, maskedAWSSecret := WithMaskedSecrets(p.APIKeys, p.AWSAccessKeyID, p.AWSSecretAccessKey)
	return ProviderResponse{
		Name:               p.Name,
		APIType:            p.APIType,
		BaseURL:            p.BaseURL,
		HealthEndpoint:     p.HealthEndpoint,
		Timeout:            p.Timeout,
		Priority:           p.Priority,
		Weight:             p.Weight,
		APIKeys:            maskedKeys,
		AuthScheme:         p.AuthScheme,
		AuthHeader:         p.AuthHeader,
		AuthPrefix:         p.AuthPrefix,
		AdditionalHeaders:  p.AdditionalHeaders,
		ProxyURL:           p.ProxyURL,
		AWSRegion:          p.AWSRegion,
		AWSAccessKeyID:     maskedAWSAccess,
		AWSSecretAccessKey: maskedAWSSecret,
		Source:             p.Source,
		Status:             status,
		LatencyMs:          latency,
		LastCheck:          lastCheck,
	}
}

type RouteRequest struct {
	Tenant    string   `json:"tenant"`
	Model     string   `json:"model" binding:"required"`
	Providers []string `json:"providers" binding:"required"`
}

type RouteResponse struct {
	Tenant    string   `json:"tenant"`
	Model     string   `json:"model"`
	Providers []string `json:"providers"`
	Source    string   `json:"source"`
}

// @sk-task model-aliases-weighted-lb#T3.1: tenant model alias DTOs (AC-009)
type AliasRequest struct {
	Tenant string `json:"tenant"`
	Alias  string `json:"alias" binding:"required"`
	Target string `json:"target"`
}

type AliasResponse struct {
	Tenant string `json:"tenant"`
	Alias  string `json:"alias"`
	Target string `json:"target"`
	Source string `json:"source"`
}
