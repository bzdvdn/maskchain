package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-task admin-ui-design#T3.1: RoutingHandler exposes providers and routing rules (AC-006)
// @sk-task 150-admin-routing-crud#T2.2: RoutingHandler reads from registry repo + CRUD (AC-001, AC-002)
//
// RoutingHandler serves routing registry data and CRUD operations.
type RoutingHandler struct {
	repo          routing.RegistryRepository
	healthChecker *ProviderHealthChecker
	auditLog      AuditLogger
}

func NewRoutingHandler(repo routing.RegistryRepository, healthChecker *ProviderHealthChecker, auditLog AuditLogger) *RoutingHandler {
	return &RoutingHandler{repo: repo, healthChecker: healthChecker, auditLog: auditLog}
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

func (h *RoutingHandler) UpsertProvider(c *gin.Context) {
	var req dto.ProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.APIType == "" {
		req.APIType = "openai"
	}
	if len(req.APIKeys) == 0 {
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
	if err := h.repo.UpsertProvider(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "upsert_provider", req.Name, map[string]any{"api_type": req.APIType, "base_url": req.BaseURL})
	c.JSON(http.StatusOK, gin.H{"data": dto.ProviderToResponse(p, "unknown", 0, 0)})
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
	if err := h.repo.UpsertRoute(c.Request.Context(), req.Tenant, req.Model, req.Providers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "upsert_route", req.Model, map[string]any{"tenant": req.Tenant, "providers": req.Providers})
	c.JSON(http.StatusOK, gin.H{"data": dto.RouteResponse{Tenant: req.Tenant, Model: req.Model, Providers: req.Providers, Source: "ui"}})
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
