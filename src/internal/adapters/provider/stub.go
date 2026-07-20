package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task 70-routing-engine#T1.3: Create stub provider client for tests (AC-002, AC-007)
//
// StubClient represents a domain entity or configuration.
type StubClient struct {
	Name            string
	FailWithStatus  int
	FailWithError   error
	ResponseBody    []byte
	ResponseHeaders map[string]string
}

func NewStubClient(name string) *StubClient {
	return &StubClient{
		Name:         name,
		ResponseBody: []byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`),
	}
}

// @sk-task 71-egress-streaming#T1.3: Add Stream() stub to StubClient (AC-003)
func (c *StubClient) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	ch := make(chan ports.ProviderChunk, 1)
	ch <- ports.ProviderChunk{Done: true}
	close(ch)
	return ch, nil
}

func (c *StubClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	if c.FailWithError != nil {
		return nil, c.FailWithError
	}
	if c.FailWithStatus != 0 {
		return &ports.ProviderResponse{
			StatusCode: c.FailWithStatus,
			Body:       []byte(fmt.Sprintf(`{"error":"stub error %d"}`, c.FailWithStatus)),
		}, nil
	}
	return &ports.ProviderResponse{
		StatusCode: http.StatusOK,
		Body:       c.ResponseBody,
		Headers:    c.ResponseHeaders,
	}, nil
}
