package ports

import "context"

// @sk-task 80-tenant-isolation#T3.1: Add Headers field for X-Tenant-ID propagation (AC-007)
// @sk-task anthropic-messages-endpoint#T1.1: Add Path field for native messages endpoint (AC-003)
//
// ProviderRequest represents a domain entity or configuration.
type ProviderRequest struct {
	Method  string
	URL     string
	Body    []byte
	Headers map[string]string
	Path    string
}

type ProviderResponse struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}

// @sk-task 70-routing-engine#T1.3: Create ProviderClient port interface (AC-002)
// @sk-task 71-egress-streaming#T1.1: Add ProviderChunk and Stream() to ProviderClient (AC-003)
//
// ProviderClient defines the interface for domain operations.
type ProviderClient interface {
	Call(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
	Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderChunk, error)
}

// @sk-task 71-egress-streaming#T1.1: Add ProviderChunk type (AC-003)
//
// ProviderChunk represents a domain entity or configuration.
type ProviderChunk struct {
	Data []byte
	Err  error
	Done bool
}
