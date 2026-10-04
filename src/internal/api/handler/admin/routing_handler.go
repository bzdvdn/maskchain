package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-task admin-ui-design#T3.1: RoutingHandler exposes providers and routing rules (AC-006)
// @sk-task 150-admin-routing-crud#T2.2: RoutingHandler reads from registry repo + CRUD (AC-001, AC-002)
//
// TxRunner runs a function inside a database transaction. Repositories are
// transaction-aware through the context, so the same ctx passed to the function
// must be forwarded to repository calls.
type TxRunner interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// ModelDiscoverer lists the models a provider exposes via its own API.
type ModelDiscoverer interface {
	Discover(ctx context.Context, p routing.ProviderConfig) ([]string, error)
}

// RoutingHandler serves routing registry data and CRUD operations.
type RoutingHandler struct {
	repo          routing.RegistryRepository
	costRates     analytics.CostRateRepository
	tx            TxRunner
	models        ModelDiscoverer
	healthChecker *ProviderHealthChecker
	auditLog      AuditLogger
}

// @sk-task routing-ia#T2.2: handler takes cost rates and a tx runner (AC-003, AC-004)
// @sk-task routing-ia#T5.1: handler takes a model discoverer (AC-011)
func NewRoutingHandler(repo routing.RegistryRepository, costRates analytics.CostRateRepository, tx TxRunner, models ModelDiscoverer, healthChecker *ProviderHealthChecker, auditLog AuditLogger) *RoutingHandler {
	return &RoutingHandler{repo: repo, costRates: costRates, tx: tx, models: models, healthChecker: healthChecker, auditLog: auditLog}
}

type modelRouteResponse struct {
	Model     string   `json:"model"`
	Tenants   []string `json:"tenants"`
	Providers []string `json:"providers"`
}

type routingResponse struct {
	Providers   []dto.ProviderResponse `json:"providers"`
	ModelRoutes []modelRouteResponse   `json:"model_routes"`
}

// @sk-task admin-ui-design#T3.1: HandleRouting returns providers and routing rules (AC-006)
func (h *RoutingHandler) HandleRouting(c *gin.Context) {
	providers, err := h.listProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load routing"})
		return
	}

	rules, err := h.repo.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load routing rules"})
		return
	}

	// group by model: collect unique tenants and providers per model
	type modelGroup struct {
		tenants   map[string]struct{}
		providers map[string]struct{}
	}
	modelGroups := make(map[string]*modelGroup)
	for _, r := range rules {
		for _, route := range r.Routes {
			mg, ok := modelGroups[route.Model]
			if !ok {
				mg = &modelGroup{
					tenants:   make(map[string]struct{}),
					providers: make(map[string]struct{}),
				}
				modelGroups[route.Model] = mg
			}
			mg.tenants[r.Tenant] = struct{}{}
			for _, p := range route.Providers {
				mg.providers[p] = struct{}{}
			}
		}
	}

	modelRoutes := make([]modelRouteResponse, 0, len(modelGroups))
	for model, mg := range modelGroups {
		tenants := make([]string, 0, len(mg.tenants))
		for t := range mg.tenants {
			tenants = append(tenants, t)
		}
		provList := make([]string, 0, len(mg.providers))
		for p := range mg.providers {
			provList = append(provList, p)
		}
		sort.Strings(tenants)
		sort.Strings(provList)
		modelRoutes = append(modelRoutes, modelRouteResponse{
			Model:     model,
			Tenants:   tenants,
			Providers: provList,
		})
	}
	sort.Slice(modelRoutes, func(i, j int) bool {
		return modelRoutes[i].Model < modelRoutes[j].Model
	})

	c.JSON(http.StatusOK, routingResponse{Providers: providers, ModelRoutes: modelRoutes})
}

func (h *RoutingHandler) listProviders(ctx context.Context) ([]dto.ProviderResponse, error) {
	providers, err := h.repo.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ProviderResponse, len(providers))
	for i, p := range providers {
		status, latency, lastCheck := h.providerStatus(p)
		out[i] = dto.ProviderToResponse(p, status, latency, lastCheck)
	}
	return out, nil
}

func (h *RoutingHandler) providerStatus(p routing.ProviderConfig) (string, int64, int64) {
	if h.healthChecker == nil {
		return "unknown", 0, 0
	}
	result := h.healthChecker.GetResult(p.Name)
	if result == nil {
		result = h.healthChecker.Check(context.Background(), ProviderTarget{
			Name: p.Name, BaseURL: p.BaseURL, HealthEndpoint: p.HealthEndpoint,
		})
	}
	return result.Status, result.LatencyMs, result.LastCheck
}

// @sk-task 150-admin-routing-crud#T2.2: CRUD endpoints (AC-001)
func (h *RoutingHandler) ListProviders(c *gin.Context) {
	providers, err := h.listProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list providers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": providers})
}

// @sk-task routing-ia#T2.2: atomic provider+models upsert with api_type-aware keys (AC-004, AC-005)
func (h *RoutingHandler) UpsertProvider(c *gin.Context) {
	var req dto.ProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.APIType == "" {
		req.APIType = "openai"
	}
	// ollama needs no key; bedrock authenticates with AWS credentials.
	if len(req.APIKeys) == 0 && req.APIType != "ollama" && req.APIType != "bedrock" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_keys must not be empty"})
		return
	}
	p := routing.ProviderConfig{
		Name:               req.Name,
		APIType:            req.APIType,
		BaseURL:            req.BaseURL,
		HealthEndpoint:     req.HealthEndpoint,
		Timeout:            req.Timeout,
		Priority:           req.Priority,
		APIKeys:            req.APIKeys,
		AuthScheme:         req.AuthScheme,
		AuthHeader:         req.AuthHeader,
		AuthPrefix:         req.AuthPrefix,
		AdditionalHeaders:  req.AdditionalHeaders,
		ProxyURL:           req.ProxyURL,
		AWSRegion:          req.AWSRegion,
		AWSAccessKeyID:     req.AWSAccessKeyID,
		AWSSecretAccessKey: req.AWSSecretAccessKey,
	}

	save := func(ctx context.Context) error {
		if err := h.repo.UpsertProvider(ctx, p); err != nil {
			return err
		}
		if len(req.Models) == 0 {
			return nil
		}

		// Existing global routes and cost rates decide append-vs-create.
		globalProviders := map[string][]string{}
		rules, err := h.repo.ListRules(ctx)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if rule.Tenant != routing.GlobalTenant {
				continue
			}
			for _, rt := range rule.Routes {
				globalProviders[rt.Model] = rt.Providers
			}
		}
		existingRates := map[string]bool{}
		if h.costRates != nil {
			rates, err := h.costRates.List(ctx)
			if err != nil {
				return err
			}
			for _, cr := range rates {
				existingRates[cr.Model] = true
			}
		}

		for _, model := range req.Models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			providers := appendUnique(globalProviders[model], p.Name)
			if err := h.repo.UpsertRoute(ctx, routing.GlobalTenant, model, providers); err != nil {
				return err
			}
			if h.costRates != nil && !existingRates[model] {
				rate, err := analytics.NewCostRate(model, 0, 0)
				if err != nil {
					return err
				}
				if err := h.costRates.Upsert(ctx, rate); err != nil {
					return err
				}
				existingRates[model] = true
			}
		}
		return nil
	}

	var err error
	if h.tx != nil {
		err = h.tx.RunInTx(c.Request.Context(), save)
	} else {
		err = save(c.Request.Context())
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "upsert_provider", req.Name, map[string]any{"api_type": req.APIType, "base_url": req.BaseURL, "models": req.Models})
	c.JSON(http.StatusOK, gin.H{"data": dto.ProviderToResponse(p, "unknown", 0, 0)})
}

// appendUnique returns items with value appended when it is not already present.
func appendUnique(items []string, value string) []string {
	out := make([]string, len(items), len(items)+1)
	copy(out, items)
	for _, item := range items {
		if item == value {
			return out
		}
	}
	return append(out, value)
}

// @sk-task routing-ia#T5.1: list a provider's models from its own API (AC-011)
func (h *RoutingHandler) ListProviderModels(c *gin.Context) {
	if h.models == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "model discovery is not configured"})
		return
	}
	providers, err := h.repo.ListProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load providers"})
		return
	}
	name := c.Param("name")
	var found *routing.ProviderConfig
	for i := range providers {
		if providers[i].Name == name {
			found = &providers[i]
			break
		}
	}
	if found == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	ids, err := h.models.Discover(c.Request.Context(), *found)
	if err != nil {
		if errors.Is(err, routing.ErrModelsUnsupported) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ids})
}

// @sk-task routing-ia#T2.1: model aggregate for the Models page (AC-003)
func (h *RoutingHandler) ListModels(c *gin.Context) {
	rules, err := h.repo.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list models"})
		return
	}

	type aggregate struct {
		input, output float64
		currency      string
		defaults      []string
		overrides     int
		source        string
	}
	models := map[string]*aggregate{}
	ensure := func(model string) *aggregate {
		a, ok := models[model]
		if !ok {
			a = &aggregate{currency: analytics.DefaultCurrency}
			models[model] = a
		}
		return a
	}

	for _, rule := range rules {
		for _, rt := range rule.Routes {
			// @sk-task provider-model-registry#T3.1: wildcard patterns are not models (AC-005)
			if routing.IsWildcardModel(rt.Model) {
				continue
			}
			a := ensure(rt.Model)
			if rule.Tenant == routing.GlobalTenant {
				a.defaults = rt.Providers
			} else {
				a.overrides++
			}
			if a.source == "" {
				a.source = rule.Source
			}
		}
	}

	if h.costRates != nil {
		rates, err := h.costRates.List(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list cost rates"})
			return
		}
		for _, cr := range rates {
			a := ensure(cr.Model)
			a.input = cr.InputPricePer1K
			a.output = cr.OutputPricePer1K
			if cr.Currency != "" {
				a.currency = cr.Currency
			}
			if a.source == "" {
				a.source = cr.Source
			}
		}
	}

	out := make([]dto.ModelAggregate, 0, len(models))
	for model, a := range models {
		out = append(out, dto.ModelAggregate{
			Model:            model,
			InputPricePer1K:  a.input,
			OutputPricePer1K: a.output,
			Currency:         a.currency,
			DefaultProviders: a.defaults,
			OverrideCount:    a.overrides,
			Source:           a.source,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *RoutingHandler) DeleteProvider(c *gin.Context) {
	name := c.Param("name")
	if err := h.repo.DeleteProvider(c.Request.Context(), name); err != nil {
		if errors.Is(err, routing.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "delete_provider", name, nil)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *RoutingHandler) ListRoutes(c *gin.Context) {
	rules, err := h.repo.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list routes"})
		return
	}
	out := make([]dto.RouteResponse, 0)
	for _, rule := range rules {
		for _, rt := range rule.Routes {
			out = append(out, dto.RouteResponse{
				Tenant: rule.Tenant, Model: rt.Model, Providers: rt.Providers, Source: rule.Source,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *RoutingHandler) UpsertRoute(c *gin.Context) {
	var req dto.RouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// @sk-task provider-model-registry#T3.2: validate wildcard + provider refs (AC-007)
	if err := h.validateRoute(c.Request.Context(), req.Model, req.Providers); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.UpsertRoute(c.Request.Context(), req.Tenant, req.Model, req.Providers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "upsert_route", req.Model, map[string]any{"tenant": req.Tenant, "providers": req.Providers})
	c.JSON(http.StatusOK, gin.H{"data": dto.RouteResponse{Tenant: req.Tenant, Model: req.Model, Providers: req.Providers, Source: "ui"}})
}

// @sk-task provider-model-registry#T3.1: remove a provider's model from the catalog (AC-008)
func (h *RoutingHandler) DeleteProviderModel(c *gin.Context) {
	name := c.Param("name")
	model := c.Param("model")
	if name == "" || model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider and model are required"})
		return
	}

	remove := func(ctx context.Context) error {
		rules, err := h.repo.ListRules(ctx)
		if err != nil {
			return err
		}
		var current []string
		found := false
		for _, rule := range rules {
			if rule.Tenant != routing.GlobalTenant {
				continue
			}
			for _, rt := range rule.Routes {
				if rt.Model == model {
					current = rt.Providers
					found = true
				}
			}
		}
		if !found {
			return routing.ErrNotFound
		}
		remaining := make([]string, 0, len(current))
		for _, p := range current {
			if p != name {
				remaining = append(remaining, p)
			}
		}
		if len(remaining) == len(current) {
			return routing.ErrNotFound
		}
		if len(remaining) == 0 {
			return h.repo.DeleteRoute(ctx, routing.GlobalTenant, model)
		}
		return h.repo.UpsertRoute(ctx, routing.GlobalTenant, model, remaining)
	}

	var err error
	if h.tx != nil {
		err = h.tx.RunInTx(c.Request.Context(), remove)
	} else {
		err = remove(c.Request.Context())
	}
	if err != nil {
		if errors.Is(err, routing.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found for provider"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "delete_provider_model", name, map[string]any{"model": model})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// @sk-task provider-model-registry#T3.2: validate wildcard routes (AC-007)
//
// validateRoute rejects malformed wildcard patterns and wildcard routes that
// reference an unknown provider.
func (h *RoutingHandler) validateRoute(ctx context.Context, model string, providers []string) error {
	if !routing.IsWildcardModel(model) {
		return nil
	}
	if strings.Count(model, "*") != 1 || (model != "*" && !strings.HasSuffix(model, "/*")) {
		return fmt.Errorf("route model %q: malformed wildcard pattern", model)
	}
	existing, err := h.repo.ListProviders(ctx)
	if err != nil {
		return err
	}
	names := make(map[string]struct{}, len(existing))
	for _, p := range existing {
		names[p.Name] = struct{}{}
	}
	for _, name := range providers {
		if _, ok := names[name]; !ok {
			return fmt.Errorf("route model %q: unknown provider %q", model, name)
		}
	}
	return nil
}

func (h *RoutingHandler) DeleteRoute(c *gin.Context) {
	var req dto.RouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.DeleteRoute(c.Request.Context(), req.Tenant, req.Model); err != nil {
		if errors.Is(err, routing.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "delete_route", req.Model, map[string]any{"tenant": req.Tenant})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// writeAudit sends an audit event async.
func (h *RoutingHandler) writeAudit(c *gin.Context, action, target string, details map[string]any) {
	if h.auditLog == nil {
		return
	}
	detailsRaw, _ := json.Marshal(details)
	username, _ := c.Get("admin_username")
	usernameStr, _ := username.(string)
	h.auditLog.Write(c.Request.Context(), &AuditEvent{
		AdminUsername: usernameStr,
		Action:        action,
		Target:        target,
		Details:       detailsRaw,
		CreatedAt:     time.Now(),
	})
}
