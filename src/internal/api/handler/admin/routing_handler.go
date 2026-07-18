package admin

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task admin-ui-design#T3.1: RoutingHandler exposes providers and routing rules (AC-006)
type RoutingHandler struct {
	cfg           *config.RoutingConfig
	healthChecker *ProviderHealthChecker
}

func NewRoutingHandler(cfg *config.RoutingConfig, healthChecker *ProviderHealthChecker) *RoutingHandler {
	return &RoutingHandler{cfg: cfg, healthChecker: healthChecker}
}

type providerResponse struct {
	Name      string `json:"name"`
	APIType   string `json:"api_type"`
	BaseURL   string `json:"base_url"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	LastCheck int64  `json:"last_check,omitempty"`
}

type modelRouteResponse struct {
	Model     string   `json:"model"`
	Tenants   []string `json:"tenants"`
	Providers []string `json:"providers"`
}

type routingResponse struct {
	Providers   []providerResponse   `json:"providers"`
	ModelRoutes []modelRouteResponse `json:"model_routes"`
}

// @sk-task admin-ui-design#T3.1: HandleRouting returns providers and routing rules (AC-006)
func (h *RoutingHandler) HandleRouting(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusOK, routingResponse{Providers: []providerResponse{}, ModelRoutes: []modelRouteResponse{}})
		return
	}

	providers := make([]providerResponse, len(h.cfg.Providers))
	for i, p := range h.cfg.Providers {
		pr := providerResponse{
			Name:    p.Name,
			APIType: p.APIType,
			BaseURL: p.BaseURL,
		}

		if h.healthChecker != nil {
			result := h.healthChecker.GetResult(p.Name)
			if result == nil {
				// first check — do it synchronously
				result = h.healthChecker.Check(c.Request.Context(), ProviderTarget{
					Name: p.Name, BaseURL: p.BaseURL, HealthEndpoint: p.HealthEndpoint,
				})
			}
			pr.Status = result.Status
			pr.LatencyMs = result.LatencyMs
			pr.LastCheck = result.LastCheck
		} else {
			pr.Status = "unknown"
		}

		providers[i] = pr
	}

	// group by model: collect unique tenants and providers per model
	type modelGroup struct {
		tenants   map[string]struct{}
		providers map[string]struct{}
	}
	modelGroups := make(map[string]*modelGroup)
	for _, r := range h.cfg.Rules {
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
		providers := make([]string, 0, len(mg.providers))
		for p := range mg.providers {
			providers = append(providers, p)
		}
		sort.Strings(tenants)
		sort.Strings(providers)
		modelRoutes = append(modelRoutes, modelRouteResponse{
			Model:     model,
			Tenants:   tenants,
			Providers: providers,
		})
	}
	sort.Slice(modelRoutes, func(i, j int) bool {
		return modelRoutes[i].Model < modelRoutes[j].Model
	})

	c.JSON(http.StatusOK, routingResponse{Providers: providers, ModelRoutes: modelRoutes})
}
