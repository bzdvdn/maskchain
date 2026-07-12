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
type ProviderClient interface {
	Call(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
}
