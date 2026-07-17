package admin

import (
	"net/http"

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

type routeResponse struct {
	Tenant    string   `json:"tenant"`
	Model     string   `json:"model"`
	Providers []string `json:"providers"`
}

type routingResponse struct {
	Providers []providerResponse `json:"providers"`
	Rules     []routeResponse    `json:"rules"`
}

// @sk-task admin-ui-design#T3.1: HandleRouting returns providers and routing rules (AC-006)
func (h *RoutingHandler) HandleRouting(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusOK, routingResponse{Providers: []providerResponse{}, Rules: []routeResponse{}})
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

	var rules []routeResponse
	for _, r := range h.cfg.Rules {
		for _, route := range r.Routes {
			rules = append(rules, routeResponse{
				Tenant:    r.Tenant,
				Model:     route.Model,
				Providers: route.Providers,
			})
		}
	}
	if rules == nil {
		rules = []routeResponse{}
	}

	c.JSON(http.StatusOK, routingResponse{Providers: providers, Rules: rules})
}
