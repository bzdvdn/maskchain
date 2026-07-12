package ports

import "context"

type ProviderRequest struct {
	Method string
	URL    string
	Body   []byte
}

type ProviderResponse struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}

// @sk-task 70-routing-engine#T1.3: Create ProviderClient port interface (AC-002)
// @sk-task 71-egress-streaming#T1.1: Add ProviderChunk and Stream() to ProviderClient (AC-003)
type ProviderClient interface {
	Call(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
	Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderChunk, error)
}

// @sk-task 71-egress-streaming#T1.1: Add ProviderChunk type (AC-003)
type ProviderChunk struct {
	Data []byte
	Err  error
	Done bool
}
