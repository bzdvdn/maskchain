package dto

import (
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-task 150-admin-routing-crud#T2.1: Routing registry DTOs (AC-001, AC-002)
//
// ProviderRequest represents a domain entity or configuration.
type ProviderRequest struct {
	Name               string            `json:"name" binding:"required"`
	APIType            string            `json:"api_type"`
	BaseURL            string            `json:"base_url" binding:"required"`
	HealthEndpoint     string            `json:"health_endpoint"`
	Timeout            string            `json:"timeout"`
	Priority           int               `json:"priority"`
	APIKeys            []string          `json:"api_keys"`
	AuthScheme         string            `json:"auth_scheme"`
	AuthHeader         string            `json:"auth_header"`
	AuthPrefix         string            `json:"auth_prefix"`
	AdditionalHeaders  map[string]string `json:"additional_headers"`
	ProxyURL           string            `json:"proxy_url"`
	AWSRegion          string            `json:"aws_region"`
	AWSAccessKeyID     string            `json:"aws_access_key_id"`
	AWSSecretAccessKey string            `json:"aws_secret_access_key"`
}

type ProviderResponse struct {
	Name               string            `json:"name"`
	APIType            string            `json:"api_type"`
	BaseURL            string            `json:"base_url"`
	HealthEndpoint     string            `json:"health_endpoint"`
	Timeout            string            `json:"timeout"`
	Priority           int               `json:"priority"`
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
	return ProviderResponse{
		Name:               p.Name,
		APIType:            p.APIType,
		BaseURL:            p.BaseURL,
		HealthEndpoint:     p.HealthEndpoint,
		Timeout:            p.Timeout,
		Priority:           p.Priority,
		APIKeys:            p.APIKeys,
		AuthScheme:         p.AuthScheme,
		AuthHeader:         p.AuthHeader,
		AuthPrefix:         p.AuthPrefix,
		AdditionalHeaders:  p.AdditionalHeaders,
		ProxyURL:           p.ProxyURL,
		AWSRegion:          p.AWSRegion,
		AWSAccessKeyID:     p.AWSAccessKeyID,
		AWSSecretAccessKey: p.AWSSecretAccessKey,
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
