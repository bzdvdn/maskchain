package api

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
)

// @sk-task api-self-service: SelfHandler exposes identity/model discovery to key holders
//
// SelfHandler backs the data-plane self-service endpoints:
//
//	GET /api/v1/models — models routed for the authenticated tenant/key
//	GET /api/v1/me     — identity, scopes and budget view for the current key
//
// Both require the virtual-key auth middleware to have resolved a tenant.
type SelfHandler struct {
	registry *routingSvc.ProviderRegistry
	version  string
}

// NewSelfHandler builds the self-service handler. registry may be nil, in which
// case /models returns an empty list.
func NewSelfHandler(registry *routingSvc.ProviderRegistry, version string) *SelfHandler {
	return &SelfHandler{registry: registry, version: version}
}

// selfModel is one entry of GET /api/v1/models.
type selfModel struct {
	ID      string `json:"id"`
	Allowed bool   `json:"allowed"`
}

// selfMeResponse is the payload of GET /api/v1/me.
type selfMeResponse struct {
	TenantID      string            `json:"tenant_id"`
	Version       string            `json:"version,omitempty"`
	KeyID         string            `json:"key_id,omitempty"`
	Label         string            `json:"label,omitempty"`
	AllowedModels []string          `json:"allowed_models,omitempty"`
	BlockedModels []string          `json:"blocked_models,omitempty"`
	BudgetCap     *float64          `json:"budget_cap,omitempty"`
	Spent         float64           `json:"spent"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// HandleModels lists the models routed for the authenticated tenant, flagging
// whether the presenting virtual key is allowed to use each one.
func (h *SelfHandler) HandleModels(c *gin.Context) {
	tenant, ok := middleware.TenantFromContext(c)
	if !ok || tenant == nil {
		middleware.AbortWithError(c, http.StatusUnauthorized, middleware.ErrorCodeUnauthorized, "unauthorized")
		return
	}

	models := h.modelsForTenant(tenant.Slug().String())

	vk, hasKey := middleware.VirtualKeyFromContext(c)
	out := make([]selfModel, 0, len(models))
	for _, m := range models {
		allowed := true
		if hasKey && vk != nil {
			allowed = vk.AllowsModel(m)
		}
		out = append(out, selfModel{ID: m, Allowed: allowed})
	}

	c.JSON(http.StatusOK, out)
}

// HandleMe returns the identity, model scopes and budget view of the current key.
func (h *SelfHandler) HandleMe(c *gin.Context) {
	tenant, ok := middleware.TenantFromContext(c)
	if !ok || tenant == nil {
		middleware.AbortWithError(c, http.StatusUnauthorized, middleware.ErrorCodeUnauthorized, "unauthorized")
		return
	}

	resp := selfMeResponse{
		TenantID: tenant.Slug().String(),
		Version:  h.version,
	}
	if vk, hasKey := middleware.VirtualKeyFromContext(c); hasKey && vk != nil {
		resp.KeyID = vk.ID
		resp.Label = vk.Label
		resp.AllowedModels = vk.AllowedModels
		resp.BlockedModels = vk.BlockedModels
		resp.BudgetCap = vk.BudgetCap
		resp.Spent = vk.Spent
		resp.ExpiresAt = vk.ExpiresAt
		resp.Metadata = vk.Metadata
	}

	c.JSON(http.StatusOK, resp)
}

// modelsForTenant returns the sorted, de-duplicated set of models routed for a
// tenant across all routing rules.
func (h *SelfHandler) modelsForTenant(tenantID string) []string {
	if h.registry == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, rule := range h.registry.Rules() {
		if rule.TenantID != tenantID {
			continue
		}
		for _, route := range rule.Routes {
			if route.Model == "" {
				continue
			}
			seen[route.Model] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
