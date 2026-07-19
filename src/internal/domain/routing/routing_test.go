package routing

import "testing"

// @sk-test release-readiness: routing entity creation and getter tests
func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		status HealthStatus
		want   string
	}{
		{HealthUnknown, "unknown"},
		{HealthHealthy, "healthy"},
		{HealthUnhealthy, "unhealthy"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("HealthStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// @sk-test release-readiness: routing entity creation and getter tests
func TestNewProvider(t *testing.T) {
	p := NewProvider("test-provider", "https://api.example.com", "/health", "30s", 1)
	if p.Name != "test-provider" {
		t.Errorf("Name = %q, want %q", p.Name, "test-provider")
	}
	if p.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want %q", p.BaseURL, "https://api.example.com")
	}
	if p.HealthEndpoint != "/health" {
		t.Errorf("HealthEndpoint = %q, want %q", p.HealthEndpoint, "/health")
	}
	if p.Timeout != "30s" {
		t.Errorf("Timeout = %q, want %q", p.Timeout, "30s")
	}
	if p.Priority != 1 {
		t.Errorf("Priority = %d, want %d", p.Priority, 1)
	}
}

// @sk-test release-readiness: routing entity creation and getter tests
func TestProvider_HealthStatus_Default(t *testing.T) {
	p := NewProvider("test", "https://example.com", "/health", "5s", 0)
	if got := p.HealthStatus(); got != HealthUnknown {
		t.Errorf("initial HealthStatus = %v, want %v", got, HealthUnknown)
	}
}

// @sk-test release-readiness: routing entity creation and getter tests
func TestProvider_SetHealthStatus(t *testing.T) {
	p := NewProvider("test", "https://example.com", "/health", "5s", 0)

	p.SetHealthStatus(HealthHealthy)
	if got := p.HealthStatus(); got != HealthHealthy {
		t.Errorf("after set healthy: got %v, want %v", got, HealthHealthy)
	}

	p.SetHealthStatus(HealthUnhealthy)
	if got := p.HealthStatus(); got != HealthUnhealthy {
		t.Errorf("after set unhealthy: got %v, want %v", got, HealthUnhealthy)
	}
}

// @sk-test release-readiness: routing entity creation and getter tests
func TestNewRoute(t *testing.T) {
	r := NewRoute("gpt-4", []string{"openai", "azure"})
	if r.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", r.Model, "gpt-4")
	}
	if len(r.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(r.Providers))
	}
	if r.Providers[0] != "openai" {
		t.Errorf("Providers[0] = %q, want %q", r.Providers[0], "openai")
	}
	if r.Providers[1] != "azure" {
		t.Errorf("Providers[1] = %q, want %q", r.Providers[1], "azure")
	}
}

// @sk-test release-readiness: routing entity creation and getter tests
func TestNewRoutingRule(t *testing.T) {
	r1 := NewRoute("gpt-4", []string{"openai"})
	r2 := NewRoute("claude-3", []string{"anthropic"})
	rule := NewRoutingRule("tenant-alpha", []Route{r1, r2})

	if rule.TenantID != "tenant-alpha" {
		t.Errorf("TenantID = %q, want %q", rule.TenantID, "tenant-alpha")
	}
	if len(rule.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(rule.Routes))
	}
	if rule.Routes[0].Model != "gpt-4" {
		t.Errorf("Routes[0].Model = %q, want %q", rule.Routes[0].Model, "gpt-4")
	}
	if rule.Routes[1].Model != "claude-3" {
		t.Errorf("Routes[1].Model = %q, want %q", rule.Routes[1].Model, "claude-3")
	}
}
