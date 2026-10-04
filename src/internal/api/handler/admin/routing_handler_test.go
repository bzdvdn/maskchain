package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// fakeRegistryRepo is an in-memory routing.RegistryRepository for handler tests.
type fakeRegistryRepo struct {
	providers          []routingDomain.ProviderConfig
	rules              []routingDomain.RuleConfig
	routes             map[string][]string
	failUpsertProvider bool
}

func routeKey(tenant, model string) string { return tenant + "/" + model }

func (f *fakeRegistryRepo) ListProviders(_ context.Context) ([]routingDomain.ProviderConfig, error) {
	out := make([]routingDomain.ProviderConfig, len(f.providers))
	copy(out, f.providers)
	return out, nil
}

func (f *fakeRegistryRepo) ListRules(_ context.Context) ([]routingDomain.RuleConfig, error) {
	return f.rules, nil
}

func (f *fakeRegistryRepo) UpsertProvider(_ context.Context, p routingDomain.ProviderConfig) error {
	if f.failUpsertProvider {
		return errors.New("boom")
	}
	for i, existing := range f.providers {
		if existing.Name == p.Name {
			f.providers[i] = p
			return nil
		}
	}
	f.providers = append(f.providers, p)
	return nil
}

func (f *fakeRegistryRepo) DeleteProvider(_ context.Context, name string) error {
	for i, p := range f.providers {
		if p.Name == name {
			f.providers = append(f.providers[:i], f.providers[i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeRegistryRepo) UpsertRoute(_ context.Context, tenant, model string, providers []string) error {
	if f.routes == nil {
		f.routes = map[string][]string{}
	}
	f.routes[routeKey(tenant, model)] = providers
	f.upsertRuleRoute(tenant, model, providers)
	return nil
}

func (f *fakeRegistryRepo) DeleteRoute(_ context.Context, tenant, model string) error {
	key := routeKey(tenant, model)
	if _, ok := f.routes[key]; !ok {
		return routingDomain.ErrNotFound
	}
	delete(f.routes, key)
	f.removeRuleRoute(tenant, model)
	return nil
}

func (f *fakeRegistryRepo) upsertRuleRoute(tenant, model string, providers []string) {
	for i := range f.rules {
		if f.rules[i].Tenant != tenant {
			continue
		}
		for j := range f.rules[i].Routes {
			if f.rules[i].Routes[j].Model == model {
				f.rules[i].Routes[j].Providers = providers
				return
			}
		}
		f.rules[i].Routes = append(f.rules[i].Routes, routingDomain.RouteConfig{Model: model, Providers: providers})
		return
	}
	f.rules = append(f.rules, routingDomain.RuleConfig{Tenant: tenant,
		Routes: []routingDomain.RouteConfig{{Model: model, Providers: providers}}})
}

func (f *fakeRegistryRepo) removeRuleRoute(tenant, model string) {
	for i := range f.rules {
		if f.rules[i].Tenant != tenant {
			continue
		}
		out := f.rules[i].Routes[:0]
		for _, rt := range f.rules[i].Routes {
			if rt.Model != model {
				out = append(out, rt)
			}
		}
		f.rules[i].Routes = out
		return
	}
}

func (f *fakeRegistryRepo) SeedFromYAML(_ context.Context, providers []routingDomain.ProviderConfig, _ []routingDomain.RuleConfig) (bool, error) {
	f.providers = providers
	return true, nil
}

// dtoProviderResponse mirrors the secret fields of dto.ProviderResponse so the
// masking test can decode the JSON body without a package import cycle.
type dtoProviderResponse struct {
	Name               string   `json:"name"`
	APIKeys            []string `json:"api_keys"`
	AWSAccessKeyID     string   `json:"aws_access_key_id"`
	AWSSecretAccessKey string   `json:"aws_secret_access_key"`
}

func hasMaskMarker(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i] == '*' && s[i+1] == '*' && s[i+2] == '*' {
			return true
		}
	}
	return false
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestRoutingHandlerListMasksSecrets (AC-003)
func TestRoutingHandlerListMasksSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRegistryRepo{providers: []routingDomain.ProviderConfig{
		{
			Name:               "openai",
			APIType:            "openai",
			BaseURL:            "https://api.openai.com/v1",
			APIKeys:            []string{"sk-longsecret-123456789"},
			AWSAccessKeyID:     "AKIAACCESSKEYID0000",
			AWSSecretAccessKey: "supersecretvalue00000",
		},
	}}
	h := NewRoutingHandler(repo, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/routing/providers", h.ListProviders)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routing/providers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []dtoProviderResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(resp.Data))
	}
	p := resp.Data[0]
	if len(p.APIKeys) != 1 || p.APIKeys[0] == "sk-longsecret-123456789" {
		t.Errorf("expected api_keys masked, got %v", p.APIKeys)
	}
	if !hasMaskMarker(p.APIKeys[0]) {
		t.Errorf("expected api_keys to contain mask marker, got %q", p.APIKeys[0])
	}
	if p.AWSAccessKeyID == "AKIAACCESSKEYID0000" {
		t.Error("expected aws_access_key_id masked")
	}
	if p.AWSSecretAccessKey == "supersecretvalue00000" {
		t.Error("expected aws_secret_access_key masked")
	}
}

// fakeCostRates is an in-memory analytics.CostRateRepository for handler tests.
type fakeCostRates struct {
	rates map[string]*analytics.CostRate
}

func (f *fakeCostRates) List(_ context.Context) ([]*analytics.CostRate, error) {
	out := make([]*analytics.CostRate, 0, len(f.rates))
	for _, r := range f.rates {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeCostRates) Upsert(_ context.Context, rate *analytics.CostRate) error {
	if f.rates == nil {
		f.rates = map[string]*analytics.CostRate{}
	}
	f.rates[rate.Model] = rate
	return nil
}

func (f *fakeCostRates) Delete(_ context.Context, model string) error {
	delete(f.rates, model)
	return nil
}

func (f *fakeCostRates) SeedFromYAML(_ context.Context, _ []*analytics.CostRate) (bool, error) {
	return false, nil
}

type fakeTx struct{ calls int }

func (f *fakeTx) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.calls++
	return fn(ctx)
}

func postProvider(t *testing.T, h *RoutingHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/routing/providers", h.UpsertProvider)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/routing/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// @sk-test routing-ia#T4.2: model aggregate (AC-003)
func TestRoutingHandlerListModelsAggregate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRegistryRepo{rules: []routingDomain.RuleConfig{
		{Tenant: routingDomain.GlobalTenant, Routes: []routingDomain.RouteConfig{{Model: "gpt-4o", Providers: []string{"openai"}}}},
		{Tenant: "alpha", Routes: []routingDomain.RouteConfig{{Model: "gpt-4o", Providers: []string{"azure"}}}},
		{Tenant: "beta", Routes: []routingDomain.RouteConfig{{Model: "gpt-4o", Providers: []string{"azure"}}}},
	}}
	costs := &fakeCostRates{rates: map[string]*analytics.CostRate{
		"gpt-4o": {Model: "gpt-4o", InputPricePer1K: 1.5, OutputPricePer1K: 2.5, Currency: "USD"},
	}}
	h := NewRoutingHandler(repo, costs, &fakeTx{}, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/routing/models", h.ListModels)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/routing/models", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []dto.ModelAggregate `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 model, got %d", len(resp.Data))
	}
	m := resp.Data[0]
	if m.Model != "gpt-4o" || m.OverrideCount != 2 {
		t.Errorf("model = %+v, want gpt-4o with 2 overrides", m)
	}
	if len(m.DefaultProviders) != 1 || m.DefaultProviders[0] != "openai" {
		t.Errorf("default providers = %v, want [openai]", m.DefaultProviders)
	}
	if m.InputPricePer1K != 1.5 || m.OutputPricePer1K != 2.5 {
		t.Errorf("cost = %v/%v, want 1.5/2.5", m.InputPricePer1K, m.OutputPricePer1K)
	}
}

// @sk-test routing-ia#T4.2: provider + models saved atomically (AC-004)
func TestRoutingHandlerUpsertProviderWithModels(t *testing.T) {
	repo := &fakeRegistryRepo{}
	costs := &fakeCostRates{}
	tx := &fakeTx{}
	h := NewRoutingHandler(repo, costs, tx, nil, nil, nil)

	w := postProvider(t, h, `{"name":"openrouter","api_type":"openai","base_url":"https://openrouter.ai/api/v1","api_keys":["sk-x"],"models":["m1","m2"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if tx.calls != 1 {
		t.Errorf("expected the write to run in a transaction, calls=%d", tx.calls)
	}
	for _, model := range []string{"m1", "m2"} {
		got := repo.routes[routeKey(routingDomain.GlobalTenant, model)]
		if len(got) != 1 || got[0] != "openrouter" {
			t.Errorf("global route %s = %v, want [openrouter]", model, got)
		}
		if costs.rates[model] == nil {
			t.Errorf("expected placeholder cost rate for %s", model)
		}
	}
}

// @sk-test routing-ia#T4.2: a failed provider write leaves no routes or rates (AC-004)
func TestRoutingHandlerUpsertProviderRollback(t *testing.T) {
	repo := &fakeRegistryRepo{failUpsertProvider: true}
	costs := &fakeCostRates{}
	h := NewRoutingHandler(repo, costs, &fakeTx{}, nil, nil, nil)

	w := postProvider(t, h, `{"name":"openrouter","api_type":"openai","base_url":"https://openrouter.ai/api/v1","api_keys":["sk-x"],"models":["m1"]}`)
	if w.Code == http.StatusOK {
		t.Fatalf("expected failure, got 200")
	}
	if len(repo.routes) != 0 {
		t.Errorf("expected no routes after failure, got %v", repo.routes)
	}
	if len(costs.rates) != 0 {
		t.Errorf("expected no cost rates after failure, got %v", costs.rates)
	}
}

// @sk-test routing-ia#T4.2: key validation is api_type-aware (AC-005)
func TestRoutingHandlerUpsertProviderKeyValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"openai without keys", `{"name":"p","api_type":"openai","base_url":"https://x"}`, http.StatusBadRequest},
		{"ollama without keys", `{"name":"p","api_type":"ollama","base_url":"http://localhost:11434"}`, http.StatusOK},
		{"bedrock without keys", `{"name":"p","api_type":"bedrock","base_url":"https://bedrock","aws_region":"us-east-1","aws_access_key_id":"a","aws_secret_access_key":"b"}`, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewRoutingHandler(&fakeRegistryRepo{}, &fakeCostRates{}, &fakeTx{}, nil, nil, nil)
			if w := postProvider(t, h, tc.body); w.Code != tc.want {
				t.Errorf("expected %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

type fakeDiscoverer struct {
	ids []string
	err error
}

func (f *fakeDiscoverer) Discover(_ context.Context, _ routingDomain.ProviderConfig) ([]string, error) {
	return f.ids, f.err
}

func getProviderModels(t *testing.T, h *RoutingHandler, name string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/routing/providers/:name/models", h.ListProviderModels)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/routing/providers/"+name+"/models", nil))
	return w
}

// @sk-test routing-ia#T5.3: provider model discovery endpoint (AC-011)
func TestRoutingHandlerListProviderModels(t *testing.T) {
	repo := &fakeRegistryRepo{providers: []routingDomain.ProviderConfig{
		{Name: "openai", APIType: "openai", BaseURL: "https://api.openai.com/v1"},
	}}

	t.Run("returns models", func(t *testing.T) {
		h := NewRoutingHandler(repo, &fakeCostRates{}, &fakeTx{}, &fakeDiscoverer{ids: []string{"a", "b"}}, nil, nil)
		w := getProviderModels(t, h, "openai")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			Data []string `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Data) != 2 || resp.Data[0] != "a" {
			t.Errorf("data = %v, want [a b]", resp.Data)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		h := NewRoutingHandler(repo, &fakeCostRates{}, &fakeTx{}, &fakeDiscoverer{err: routingDomain.ErrModelsUnsupported}, nil, nil)
		if w := getProviderModels(t, h, "openai"); w.Code != http.StatusNotImplemented {
			t.Errorf("expected 501, got %d", w.Code)
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		h := NewRoutingHandler(repo, &fakeCostRates{}, &fakeTx{}, &fakeDiscoverer{}, nil, nil)
		if w := getProviderModels(t, h, "missing"); w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func deleteProviderModel(t *testing.T, h *RoutingHandler, name, model string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/routing/providers/:name/models/:model", h.DeleteProviderModel)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete,
		"/api/v1/routing/providers/"+name+"/models/"+model, nil))
	return w
}

// @sk-test provider-model-registry#T3.3: removing a provider from a shared route keeps the route (AC-008)
func TestRoutingHandlerDeleteProviderModel(t *testing.T) {
	repo := &fakeRegistryRepo{
		routes: map[string][]string{routeKey(routingDomain.GlobalTenant, "m1"): {"openrouter", "openai"}},
		rules: []routingDomain.RuleConfig{
			{Tenant: routingDomain.GlobalTenant, Routes: []routingDomain.RouteConfig{
				{Model: "m1", Providers: []string{"openrouter", "openai"}},
			}},
		},
	}
	costs := &fakeCostRates{}
	h := NewRoutingHandler(repo, costs, &fakeTx{}, nil, nil, nil)

	if w := deleteProviderModel(t, h, "openai", "m1"); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got := repo.routes[routeKey(routingDomain.GlobalTenant, "m1")]
	if len(got) != 1 || got[0] != "openrouter" {
		t.Fatalf("remaining providers = %v, want [openrouter]", got)
	}

	// Removing the last provider deletes the route entirely.
	if w := deleteProviderModel(t, h, "openrouter", "m1"); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := repo.routes[routeKey(routingDomain.GlobalTenant, "m1")]; ok {
		t.Errorf("route should be deleted when no providers remain")
	}

	if w := deleteProviderModel(t, h, "openai", "missing"); w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown model, got %d", w.Code)
	}
}

// @sk-test provider-model-registry#T3.3: wildcard route validation (AC-007)
func TestRoutingHandlerUpsertRouteWildcardValidation(t *testing.T) {
	putRoute := func(t *testing.T, h *RoutingHandler, body string) *httptest.ResponseRecorder {
		t.Helper()
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PUT("/api/v1/routing/routes", h.UpsertRoute)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/routing/routes", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	repo := &fakeRegistryRepo{providers: []routingDomain.ProviderConfig{{Name: "groq", BaseURL: "http://groq"}}}
	h := NewRoutingHandler(repo, &fakeCostRates{}, &fakeTx{}, nil, nil, nil)

	if w := putRoute(t, h, `{"tenant":"*","model":"groq/*","providers":["groq"]}`); w.Code != http.StatusOK {
		t.Errorf("valid wildcard: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := putRoute(t, h, `{"tenant":"*","model":"foo*bar","providers":["groq"]}`); w.Code != http.StatusBadRequest {
		t.Errorf("malformed pattern: expected 400, got %d", w.Code)
	}
	if w := putRoute(t, h, `{"tenant":"*","model":"groq/*","providers":["nope"]}`); w.Code != http.StatusBadRequest {
		t.Errorf("unknown provider: expected 400, got %d", w.Code)
	}
}
