package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// selfFakeRepo is a minimal in-memory VirtualKeyRepository for handler tests.
type selfFakeRepo struct {
	keys []*virtualkey.VirtualKey
}

func (f *selfFakeRepo) FindByKeyHash(_ context.Context, hash string) (*virtualkey.VirtualKey, error) {
	for _, k := range f.keys {
		if k.KeyHash == hash {
			return k, nil
		}
	}
	return nil, virtualkey.ErrNotFound
}

func (f *selfFakeRepo) GetById(_ context.Context, _ string) (*virtualkey.VirtualKey, error) {
	return nil, virtualkey.ErrNotFound
}
func (f *selfFakeRepo) ListByTenant(_ context.Context, _ string) ([]*virtualkey.VirtualKey, error) {
	return nil, nil
}
func (f *selfFakeRepo) List(_ context.Context) ([]*virtualkey.VirtualKey, error) { return f.keys, nil }

// @sk-task ui-production-readiness#T7.1: DB-level pagination/search (AC-008)
func (f *selfFakeRepo) ListPaged(_ context.Context, limit, offset int, _ string) ([]*virtualkey.VirtualKey, int, error) {
	total := len(f.keys)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return f.keys[start:end], total, nil
}
func (f *selfFakeRepo) Create(_ context.Context, _ *virtualkey.VirtualKey) error { return nil }
func (f *selfFakeRepo) Update(_ context.Context, _ *virtualkey.VirtualKey) error { return nil }
func (f *selfFakeRepo) Delete(_ context.Context, _ string) error                 { return nil }
func (f *selfFakeRepo) BackfillFromLegacy(_ context.Context, _ map[string][]string) (int, error) {
	return 0, nil
}

func selfRegistry(t *testing.T, tenantID string, models ...string) *routingSvc.ProviderRegistry {
	t.Helper()
	routes := make([]routingDomain.RouteConfig, 0, len(models))
	for _, m := range models {
		routes = append(routes, routingDomain.RouteConfig{Model: m, Providers: []string{"openai"}})
	}
	reg, err := routingSvc.NewProviderRegistry(&routingDomain.RoutingConfig{
		Providers: []routingDomain.ProviderConfig{{Name: "openai", BaseURL: "http://provider"}},
		Rules:     []routingDomain.RuleConfig{{Tenant: tenantID, Routes: routes}},
	})
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	return reg
}

func selfEngine(t *testing.T, repo *selfFakeRepo, tenantID, authHeader string) *gin.Engine {
	t.Helper()
	slug, err := value.NewTenantSlug(tenantID)
	if err != nil {
		t.Fatalf("slug: %v", err)
	}
	tenant := entity.NewTenant(slug, "Acme", authHeader)
	provider := middleware.NewTenantProvider([]*entity.Tenant{tenant})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.VirtualKeyAuth(repo, provider))
	return engine
}

// @sk-test api-self-service: TestSelfHandlerModels (AC-001, AC-002)
func TestSelfHandlerModels(t *testing.T) {
	cap := 10.0
	repo := &selfFakeRepo{keys: []*virtualkey.VirtualKey{{
		ID:            "k1",
		TenantID:      "acme",
		KeyHash:       virtualkey.KeyHash("sk-mc_secret"),
		AllowedModels: []string{"gpt-4o-mini"},
		BudgetCap:     &cap,
		Spent:         1.5,
		Enabled:       true,
	}}}

	engine := selfEngine(t, repo, "acme", "Authorization")
	h := NewSelfHandler(selfRegistry(t, "acme", "gpt-4o", "gpt-4o-mini"), "1.2.3")
	engine.GET("/api/v1/models", h.HandleModels)
	engine.GET("/api/v1/me", h.HandleMe)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-mc_secret")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var models []struct {
		ID      string `json:"id"`
		Allowed bool   `json:"allowed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &models); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d: %v", len(models), models)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	if models[0].ID != "gpt-4o" || models[0].Allowed {
		t.Errorf("expected gpt-4o disallowed, got %+v", models[0])
	}
	if models[1].ID != "gpt-4o-mini" || !models[1].Allowed {
		t.Errorf("expected gpt-4o-mini allowed, got %+v", models[1])
	}
}

// @sk-test api-self-service: TestSelfHandlerMe (AC-002)
func TestSelfHandlerMe(t *testing.T) {
	cap := 10.0
	repo := &selfFakeRepo{keys: []*virtualkey.VirtualKey{{
		ID:            "k1",
		TenantID:      "acme",
		KeyHash:       virtualkey.KeyHash("sk-mc_secret"),
		Label:         "prod-app",
		AllowedModels: []string{"gpt-4o-mini"},
		BudgetCap:     &cap,
		Spent:         1.5,
		Enabled:       true,
	}}}

	engine := selfEngine(t, repo, "acme", "Authorization")
	h := NewSelfHandler(selfRegistry(t, "acme", "gpt-4o-mini"), "1.2.3")
	engine.GET("/api/v1/me", h.HandleMe)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer sk-mc_secret")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		TenantID      string   `json:"tenant_id"`
		Version       string   `json:"version"`
		KeyID         string   `json:"key_id"`
		Label         string   `json:"label"`
		AllowedModels []string `json:"allowed_models"`
		BudgetCap     *float64 `json:"budget_cap"`
		Spent         float64  `json:"spent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if resp.TenantID != "acme" || resp.KeyID != "k1" || resp.Label != "prod-app" {
		t.Errorf("unexpected identity: %+v", resp)
	}
	if resp.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", resp.Version)
	}
	if resp.BudgetCap == nil || *resp.BudgetCap != 10 {
		t.Errorf("expected budget cap 10, got %v", resp.BudgetCap)
	}
	if resp.Spent != 1.5 {
		t.Errorf("expected spent 1.5, got %v", resp.Spent)
	}
}

// @sk-test api-self-service: TestSelfHandlerUnauthorized (AC-001)
func TestSelfHandlerUnauthorized(t *testing.T) {
	repo := &selfFakeRepo{}
	engine := selfEngine(t, repo, "acme", "Authorization")
	h := NewSelfHandler(selfRegistry(t, "acme", "gpt-4o-mini"), "1.2.3")
	engine.GET("/api/v1/models", h.HandleModels)
	engine.GET("/api/v1/me", h.HandleMe)

	for _, path := range []string{"/api/v1/models", "/api/v1/me"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", path, w.Code)
		}
	}
}

// @sk-test openai-endpoint-coverage#T3.4: GET /v1/models OpenAI shape + key scope (AC-001, AC-002)
func TestSelfHandlerModelsOpenAI(t *testing.T) {
	type openAIModelList struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	fetch := func(t *testing.T, repo *selfFakeRepo) openAIModelList {
		t.Helper()
		engine := selfEngine(t, repo, "acme", "Authorization")
		h := NewSelfHandler(selfRegistry(t, "acme", "gpt-4o", "gpt-4o-mini"), "1.2.3")
		engine.GET("/v1/models", h.HandleModelsOpenAI)

		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer sk-mc_secret")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var list openAIModelList
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			t.Fatalf("decode: %v (%s)", err, w.Body.String())
		}
		return list
	}

	t.Run("scoped key sees only allowed models", func(t *testing.T) {
		repo := &selfFakeRepo{keys: []*virtualkey.VirtualKey{{
			ID: "k1", TenantID: "acme", KeyHash: virtualkey.KeyHash("sk-mc_secret"),
			AllowedModels: []string{"gpt-4o-mini"}, Enabled: true,
		}}}
		list := fetch(t, repo)
		if list.Object != "list" {
			t.Errorf("object = %q, want list", list.Object)
		}
		if len(list.Data) != 1 || list.Data[0].ID != "gpt-4o-mini" {
			t.Fatalf("data = %+v, want only gpt-4o-mini", list.Data)
		}
		if list.Data[0].Object != "model" || list.Data[0].OwnedBy != "maskchain" || list.Data[0].Created != 0 {
			t.Errorf("unexpected entry fields: %+v", list.Data[0])
		}
	})

	t.Run("unscoped key sees all routed models", func(t *testing.T) {
		repo := &selfFakeRepo{keys: []*virtualkey.VirtualKey{{
			ID: "k1", TenantID: "acme", KeyHash: virtualkey.KeyHash("sk-mc_secret"), Enabled: true,
		}}}
		list := fetch(t, repo)
		if len(list.Data) != 2 {
			t.Fatalf("data = %+v, want 2 models", list.Data)
		}
	})
}

// @sk-test provider-model-registry#T1.4: catalog unions tenant + global routes, excludes patterns (AC-005)
func TestSelfHandlerModelsUnionIncludesGlobal(t *testing.T) {
	reg, err := routingSvc.NewProviderRegistry(&routingDomain.RoutingConfig{
		Providers: []routingDomain.ProviderConfig{{Name: "openai", BaseURL: "http://provider"}},
		Rules: []routingDomain.RuleConfig{
			{Tenant: "acme", Routes: []routingDomain.RouteConfig{
				{Model: "m1", Providers: []string{"openai"}},
				{Model: "groq/*", Providers: []string{"openai"}},
			}},
			{Tenant: routingDomain.GlobalTenant, Routes: []routingDomain.RouteConfig{
				{Model: "m2", Providers: []string{"openai"}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}

	repo := &selfFakeRepo{keys: []*virtualkey.VirtualKey{{
		ID: "k1", TenantID: "acme", KeyHash: virtualkey.KeyHash("sk-mc_secret"), Enabled: true,
	}}}
	engine := selfEngine(t, repo, "acme", "Authorization")
	h := NewSelfHandler(reg, "1.2.3")
	engine.GET("/v1/models", h.HandleModelsOpenAI)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-mc_secret")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	ids := make([]string, 0, len(list.Data))
	for _, d := range list.Data {
		ids = append(ids, d.ID)
	}
	sort.Strings(ids)
	if len(ids) != 2 || ids[0] != "m1" || ids[1] != "m2" {
		t.Fatalf("ids = %v, want [m1 m2]", ids)
	}
	for _, id := range ids {
		if id == "groq/*" {
			t.Errorf("wildcard pattern must not appear as a model")
		}
	}
}
