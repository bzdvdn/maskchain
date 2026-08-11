package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
)

// @sk-task 150-admin-routing-crud#T2.3: CostRateHandler CRUD (AC-002)
//
// CostRateHandler serves cost rate CRUD operations.
type CostRateHandler struct {
	repo     analytics.CostRateRepository
	auditLog AuditLogger
}

func NewCostRateHandler(repo analytics.CostRateRepository, auditLog AuditLogger) *CostRateHandler {
	return &CostRateHandler{repo: repo, auditLog: auditLog}
}

type costRateRequest struct {
	Model            string  `json:"model" binding:"required"`
	InputPricePer1K  float64 `json:"input_price_per_1k"`
	OutputPricePer1K float64 `json:"output_price_per_1k"`
	Currency         string  `json:"currency"`
}

type costRateResponse struct {
	Model            string  `json:"model"`
	InputPricePer1K  float64 `json:"input_price_per_1k"`
	OutputPricePer1K float64 `json:"output_price_per_1k"`
	Currency         string  `json:"currency"`
	Source           string  `json:"source"`
}

func (h *CostRateHandler) List(c *gin.Context) {
	rates, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list cost rates"})
		return
	}
	out := make([]costRateResponse, len(rates))
	for i, r := range rates {
		out[i] = costRateResponse{
			Model: r.Model, InputPricePer1K: r.InputPricePer1K, OutputPricePer1K: r.OutputPricePer1K,
			Currency: r.Currency, Source: r.Source,
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *CostRateHandler) Upsert(c *gin.Context) {
	var req costRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rate, err := analytics.NewCostRateWithCurrency(req.Model, req.InputPricePer1K, req.OutputPricePer1K, req.Currency)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Upsert(c.Request.Context(), rate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeCostRateAudit(c, "upsert_cost_rate", req.Model, map[string]any{
		"input_price_per_1k": req.InputPricePer1K, "output_price_per_1k": req.OutputPricePer1K, "currency": req.Currency,
	})
	c.JSON(http.StatusOK, gin.H{"data": costRateResponse{
		Model: req.Model, InputPricePer1K: req.InputPricePer1K, OutputPricePer1K: req.OutputPricePer1K,
		Currency: req.Currency, Source: "ui",
	}})
}

func (h *CostRateHandler) Delete(c *gin.Context) {
	model := c.Param("model")
	if err := h.repo.Delete(c.Request.Context(), model); err != nil {
		if errors.Is(err, analytics.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "cost rate not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeCostRateAudit(c, "delete_cost_rate", model, nil)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *CostRateHandler) writeCostRateAudit(c *gin.Context, action, target string, details map[string]any) {
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
