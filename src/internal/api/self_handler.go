package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
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

// @sk-task openai-endpoint-coverage#T3.2: OpenAI-shaped model list (AC-001, AC-002)
//
// openAIModel is one entry of GET /v1/models.
type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// openAIModelList is the OpenAI list envelope.
type openAIModelList struct {
	Object string        `json:"object"`
	Data   []openAIModel `json:"data"`
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

// @sk-task openai-endpoint-coverage#T3.2: GET /v1/models in the OpenAI shape (AC-001, AC-002)
//
// HandleModelsOpenAI lists the models the presenting key is allowed to use in
// the OpenAI list shape. An unscoped key sees every model routed to the tenant;
// a scoped key sees only its allowed models.
func (h *SelfHandler) HandleModelsOpenAI(c *gin.Context) {
	tenant, ok := middleware.TenantFromContext(c)
	if !ok || tenant == nil {
		middleware.AbortWithError(c, http.StatusUnauthorized, middleware.ErrorCodeUnauthorized, "unauthorized")
		return
	}

	vk, hasKey := middleware.VirtualKeyFromContext(c)
	models := h.modelsForTenant(tenant.Slug().String())
	data := make([]openAIModel, 0, len(models))
	for _, m := range models {
		if hasKey && vk != nil && !vk.AllowsModel(m) {
			continue
		}
		data = append(data, openAIModel{ID: m, Object: "model", Created: 0, OwnedBy: "maskchain"})
	}

	c.JSON(http.StatusOK, openAIModelList{Object: "list", Data: data})
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

// @sk-task provider-model-registry#T1.3: include global catalog models, exclude patterns (AC-005)
//
// modelsForTenant returns the sorted, de-duplicated set of models a tenant can
// use: its own exact routes plus the global (fallback) exact routes. Wildcard
// route patterns are excluded because they do not name a single model.
func (h *SelfHandler) modelsForTenant(tenantID string) []string {
	if h.registry == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, rule := range h.registry.Rules() {
		if rule.TenantID != tenantID && rule.TenantID != routingDomain.GlobalTenant {
			continue
		}
		for _, route := range rule.Routes {
			if route.Model == "" || strings.Contains(route.Model, "*") {
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
