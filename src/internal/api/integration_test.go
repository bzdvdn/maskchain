package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	routing2 "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

type integrationVirtualKeyRepo struct {
	keys []*virtualkey.VirtualKey
}

func (r *integrationVirtualKeyRepo) FindByKeyHash(_ context.Context, hash string) (*virtualkey.VirtualKey, error) {
	for _, k := range r.keys {
		if k.KeyHash == hash && k.Enabled {
			return k, nil
		}
	}
	return nil, virtualkey.ErrNotFound
}

func (r *integrationVirtualKeyRepo) GetById(_ context.Context, id string) (*virtualkey.VirtualKey, error) {
	for _, k := range r.keys {
		if k.ID == id {
			return k, nil
		}
	}
	return nil, virtualkey.ErrNotFound
}

func (r *integrationVirtualKeyRepo) ListByTenant(_ context.Context, tenantID string) ([]*virtualkey.VirtualKey, error) {
	var out []*virtualkey.VirtualKey
	for _, k := range r.keys {
		if k.TenantID == tenantID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (r *integrationVirtualKeyRepo) List(_ context.Context) ([]*virtualkey.VirtualKey, error) {
	return r.keys, nil
}

func (r *integrationVirtualKeyRepo) Create(_ context.Context, k *virtualkey.VirtualKey) error {
	r.keys = append(r.keys, k)
	return nil
}

func (r *integrationVirtualKeyRepo) Update(_ context.Context, k *virtualkey.VirtualKey) error {
	for i, item := range r.keys {
		if item.ID == k.ID {
			r.keys[i] = k
			return nil
		}
	}
	return virtualkey.ErrNotFound
}

func (r *integrationVirtualKeyRepo) Delete(_ context.Context, id string) error {
	for i, item := range r.keys {
		if item.ID == id {
			r.keys = append(r.keys[:i], r.keys[i+1:]...)
			return nil
		}
	}
	return virtualkey.ErrNotFound
}

func (r *integrationVirtualKeyRepo) BackfillFromLegacy(_ context.Context, _ map[string][]string) (int, error) {
	return 0, nil
}

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
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	slug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(slug, "test-tenant", "Authorization",
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules:   []entity.PIARule{{Label: "test", Type: "regex", Pattern: "NOMATCH", Action: "block"}},
		}),
	)

	engine.Use(middleware.RequestID())
	engine.Use(middleware.VirtualKeyAuth(
		&integrationVirtualKeyRepo{keys: []*virtualkey.VirtualKey{
			{ID: "k1", TenantID: "test-tenant", KeyHash: virtualkey.KeyHash("valid-key"), Enabled: true},
		}},
		middleware.NewTenantProvider([]*entity.Tenant{tenant}),
	))

	scanner := &integrationMockScanner{
		resp: &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusClean),
		},
	}
	engine.Use(middleware.ShieldMiddleware(scanner, &config.ShieldConfig{}, log))

	domainCfg := &routing2.RoutingConfig{
		Providers: []routing2.ProviderConfig{
			{Name: "primary", BaseURL: "http://localhost:1"},
		},
		Rules: []routing2.RuleConfig{
			{
				Tenant: "test-tenant",
				Routes: []routing2.RouteConfig{
					{Model: "gpt-4", Providers: []string{"primary"}},
				},
			},
		},
	}

	reg, _ := routingSvc.NewProviderRegistry(domainCfg)
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
