package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

type integrationMockScanner struct {
	resp *appshield.ScanResponse
	err  error
}

func (m *integrationMockScanner) Scan(_ context.Context, _ appshield.ScanRequest) (*appshield.ScanResponse, error) {
	return m.resp, m.err
}

type integrationMockClient struct {
	statusCode int
}

func (m *integrationMockClient) Call(_ context.Context, _ *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	return &ports.ProviderResponse{
		StatusCode: m.statusCode,
		Body:       []byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`),
	}, nil
}

func (m *integrationMockClient) Stream(_ context.Context, _ *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	ch := make(chan ports.ProviderChunk, 1)
	ch <- ports.ProviderChunk{Done: true}
	close(ch)
	return ch, nil
}

// @sk-test 117-critical-test-coverage#T3.4: TestIntegration_FullCycle (AC-005)
func TestIntegration_FullCycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	log, _ := zap.NewProduction()

	slug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(slug, "test-tenant", "Authorization", []string{"valid-key"},
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules:   []entity.PIARule{{Label: "test", Type: "regex", Pattern: "NOMATCH", Action: "block"}},
		}),
	)

	engine.Use(middleware.RequestID())
	engine.Use(middleware.Auth([]*entity.Tenant{tenant}))

	scanner := &integrationMockScanner{
		resp: &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusClean, nil),
		},
	}
	engine.Use(middleware.ShieldMiddleware(scanner, &config.ShieldConfig{}, log))

	cfg := &config.RoutingConfig{
		Providers: []config.ProviderConfig{
			{Name: "primary", BaseURL: "http://localhost:1"},
		},
		Rules: []config.RuleConfig{
			{
				Tenant: "test-tenant",
				Routes: []config.RouteConfig{
					{Model: "gpt-4", Providers: []string{"primary"}},
				},
			},
		},
	}

	reg, _ := routingSvc.NewProviderRegistry(cfg)
	sel := routingSvc.NewRouteSelector(reg)
	clients := map[string]ports.ProviderClient{
		"primary": &integrationMockClient{statusCode: http.StatusOK},
	}
	fb := routingSvc.NewFallbackHandler(clients)
	routingHandler := NewRoutingProxyHandler(sel, fb)

	engine.POST("/v1/chat/completions", middleware.WrapSSE(), routingHandler.HandleChatCompletion)

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-key")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if w.Header().Get("X-Shield-Status") != "clean" {
		t.Errorf("expected X-Shield-Status: clean, got %s", w.Header().Get("X-Shield-Status"))
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header")
	}
}
