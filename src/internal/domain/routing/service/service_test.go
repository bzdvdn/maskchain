package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-test 70-routing-engine#T4.1: TestProviderRegistry (AC-001)
func TestProviderRegistry(t *testing.T) {
	cfg := &config.RoutingConfig{
		Providers: []config.ProviderConfig{
			{Name: "openai", BaseURL: "https://api.openai.com", Timeout: "30s"},
			{Name: "azure", BaseURL: "https://azure.openai.com", Timeout: "30s"},
		},
	}

	reg, err := NewProviderRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := reg.Get("openai")
	if p == nil {
		t.Fatal("expected provider openai, got nil")
	}
	if p.Name != "openai" {
		t.Errorf("expected name openai, got %s", p.Name)
	}

	pNil := reg.Get("nonexistent")
	if pNil != nil {
		t.Errorf("expected nil for nonexistent provider, got %v", pNil)
	}

	list := reg.List()
	if len(list) != 2 {
		t.Errorf("expected 2 providers, got %d", len(list))
	}
}

// @sk-test 70-routing-engine#T4.1: TestProviderRegistryNilConfig (AC-001)
func TestProviderRegistryNilConfig(t *testing.T) {
	reg, err := NewProviderRegistry(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list := reg.List()
	if len(list) != 0 {
		t.Errorf("expected 0 providers for nil config, got %d", len(list))
	}
}

// @sk-test 70-routing-engine#T4.1: TestRouteSelector (AC-001, AC-004)
func TestRouteSelector(t *testing.T) {
	cfg := &config.RoutingConfig{
		Providers: []config.ProviderConfig{
			{Name: "openai", BaseURL: "https://api.openai.com"},
			{Name: "azure", BaseURL: "https://azure.openai.com"},
		},
		Rules: []config.RuleConfig{
			{
				Tenant: "default",
				Routes: []config.RouteConfig{
					{Model: "gpt-4", Providers: []string{"openai"}},
				},
			},
		},
	}

	reg, _ := NewProviderRegistry(cfg)
	sel := NewRouteSelector(reg)

	p, providers, err := sel.Select("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "openai" {
		t.Errorf("expected openai, got %s", p.Name)
	}
	if len(providers) != 1 || providers[0] != "openai" {
		t.Errorf("expected providers [openai], got %v", providers)
	}

	// Test unknown model -> ErrNoRoute
	_, _, err = sel.Select("unknown-model", "")
	if !errors.Is(err, ErrNoRoute) {
		t.Errorf("expected ErrNoRoute, got %v", err)
	}
}

// @sk-test 70-routing-engine#T4.1: TestRouteSelectorSkipsUnhealthy (AC-003)
func TestRouteSelectorSkipsUnhealthy(t *testing.T) {
	cfg := &config.RoutingConfig{
		Providers: []config.ProviderConfig{
			{Name: "openai", BaseURL: "https://api.openai.com"},
			{Name: "azure", BaseURL: "https://azure.openai.com"},
		},
		Rules: []config.RuleConfig{
			{
				Tenant: "default",
				Routes: []config.RouteConfig{
					{Model: "gpt-4", Providers: []string{"openai", "azure"}},
				},
			},
		},
	}

	reg, _ := NewProviderRegistry(cfg)
	reg.Get("openai").SetHealthStatus(routing.HealthUnhealthy)

	sel := NewRouteSelector(reg)

	p, _, err := sel.Select("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "azure" {
		t.Errorf("expected azure (fallback to healthy), got %s", p.Name)
	}

	// Make all unhealthy
	reg.Get("azure").SetHealthStatus(routing.HealthUnhealthy)
	_, _, err = sel.Select("gpt-4", "")
	if !errors.Is(err, ErrNoHealthyProvider) {
		t.Errorf("expected ErrNoHealthyProvider, got %v", err)
	}
}

// @sk-test 70-routing-engine#T4.1: TestRouteSelectorTenantScoped (AC-005)
func TestRouteSelectorTenantScoped(t *testing.T) {
	cfg := &config.RoutingConfig{
		Providers: []config.ProviderConfig{
			{Name: "openai", BaseURL: "https://api.openai.com"},
			{Name: "azure", BaseURL: "https://azure.openai.com"},
		},
		Rules: []config.RuleConfig{
			{
				Tenant: "alpha",
				Routes: []config.RouteConfig{
					{Model: "gpt-4", Providers: []string{"openai"}},
				},
			},
			{
				Tenant: "beta",
				Routes: []config.RouteConfig{
					{Model: "gpt-4", Providers: []string{"azure"}},
				},
			},
		},
	}

	reg, _ := NewProviderRegistry(cfg)
	sel := NewRouteSelector(reg)

	p, _, _ := sel.Select("gpt-4", "alpha")
	if p.Name != "openai" {
		t.Errorf("expected openai for alpha, got %s", p.Name)
	}

	p, _, _ = sel.Select("gpt-4", "beta")
	if p.Name != "azure" {
		t.Errorf("expected azure for beta, got %s", p.Name)
	}
}

type mockProviderClient struct {
	name       string
	statusCode int
	err        error
}

// @sk-test 70-routing-engine#T4.1: TestFallbackHandler (AC-002, AC-007)
func (m *mockProviderClient) Call(_ context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ports.ProviderResponse{
		StatusCode: m.statusCode,
		Body:       []byte("ok"),
	}, nil
}

func (m *mockProviderClient) Stream(_ context.Context, _ *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	ch := make(chan ports.ProviderChunk, 1)
	ch <- ports.ProviderChunk{Done: true}
	close(ch)
	return ch, nil
}

func TestFallbackHandler(t *testing.T) {
	clients := map[string]ports.ProviderClient{
		"p1": &mockProviderClient{name: "p1", statusCode: http.StatusOK},
		"p2": &mockProviderClient{name: "p2", statusCode: http.StatusOK},
	}

	fb := NewFallbackHandler(clients)

	resp, name, err := fb.Call(context.Background(), []string{"p1", "p2"}, &ports.ProviderRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "p1" {
		t.Errorf("expected p1, got %s", name)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// @sk-test 70-routing-engine#T4.1: TestFallbackHandlerRetryOn5xx (AC-002, AC-007)
func TestFallbackHandlerRetryOn5xx(t *testing.T) {
	clients := map[string]ports.ProviderClient{
		"p1": &mockProviderClient{name: "p1", statusCode: http.StatusServiceUnavailable},
		"p2": &mockProviderClient{name: "p2", statusCode: http.StatusOK},
	}

	fb := NewFallbackHandler(clients)

	resp, name, err := fb.Call(context.Background(), []string{"p1", "p2"}, &ports.ProviderRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "p2" {
		t.Errorf("expected p2 (fallback), got %s", name)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// @sk-test 70-routing-engine#T4.1: TestFallbackHandlerAllFail (AC-007)
func TestFallbackHandlerAllFail(t *testing.T) {
	clients := map[string]ports.ProviderClient{
		"p1": &mockProviderClient{name: "p1", statusCode: http.StatusServiceUnavailable},
		"p2": &mockProviderClient{name: "p2", statusCode: http.StatusServiceUnavailable},
	}

	fb := NewFallbackHandler(clients)

	_, _, err := fb.Call(context.Background(), []string{"p1", "p2"}, &ports.ProviderRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// @sk-test 70-routing-engine#T4.1: TestFallbackHandlerNoRetryOn4xx (AC-002)
func TestFallbackHandlerNoRetryOn4xx(t *testing.T) {
	clients := map[string]ports.ProviderClient{
		"p1": &mockProviderClient{name: "p1", statusCode: http.StatusBadRequest},
		"p2": &mockProviderClient{name: "p2", statusCode: http.StatusOK},
	}

	fb := NewFallbackHandler(clients)

	// 4xx should not trigger fallback — p1 returns immediately
	resp, name, err := fb.Call(context.Background(), []string{"p1", "p2"}, &ports.ProviderRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "p1" {
		t.Errorf("expected p1 (4xx not retriable), got %s", name)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// @sk-test 70-routing-engine#T4.1: TestHealthChecker (AC-006)
func TestHealthChecker(t *testing.T) {
	healthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthySrv.Close()

	cfg := &config.RoutingConfig{
		Providers: []config.ProviderConfig{
			{Name: "p1", BaseURL: healthySrv.URL, HealthEndpoint: "/health"},
		},
	}

	reg, _ := NewProviderRegistry(cfg)
	reg.Get("p1").SetHealthStatus(routing.HealthUnknown)

	hc := NewHealthChecker(reg, nil)
	hc.checkAll()

	if status := reg.Get("p1").HealthStatus(); status != routing.HealthHealthy {
		t.Errorf("expected healthy, got %v", status)
	}
}

// @sk-test 70-routing-engine#T4.1: TestHealthCheckerUnhealthyEndpoint (AC-006)
func TestHealthCheckerUnhealthyEndpoint(t *testing.T) {
	unhealthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unhealthySrv.Close()

	cfg := &config.RoutingConfig{
		Providers: []config.ProviderConfig{
			{Name: "p1", BaseURL: unhealthySrv.URL, HealthEndpoint: "/health"},
		},
	}

	reg, _ := NewProviderRegistry(cfg)
	hc := NewHealthChecker(reg, nil)
	hc.checkAll()

	if status := reg.Get("p1").HealthStatus(); status != routing.HealthUnhealthy {
		t.Errorf("expected unhealthy, got %v", status)
	}
}

// @sk-test 70-routing-engine#T4.1: TestHealthCheckerNoEndpoint (AC-006)
func TestHealthCheckerNoEndpoint(t *testing.T) {
	cfg := &config.RoutingConfig{
		Providers: []config.ProviderConfig{
			{Name: "p1", BaseURL: "http://localhost:1", Timeout: "10ms"},
		},
	}

	reg, _ := NewProviderRegistry(cfg)
	reg.Get("p1").SetHealthStatus(routing.HealthUnknown)

	hc := NewHealthChecker(reg, &http.Client{Timeout: 10 * time.Millisecond})
	hc.checkAll()

	// No health endpoint -> always healthy regardless of reachability
	if status := reg.Get("p1").HealthStatus(); status != routing.HealthHealthy {
		t.Errorf("expected healthy (no endpoint), got %v", status)
	}
}
