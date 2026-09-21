package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
)

// @sk-task 301-budget-enforcement#T3.1: BudgetHandler manages budgets via admin API (AC-006)
type BudgetHandler struct {
	repo     budget.BudgetRepository
	counter  budget.SpendCounter
	auditLog AuditLogger
}

func NewBudgetHandler(repo budget.BudgetRepository, counter budget.SpendCounter, auditLog AuditLogger) *BudgetHandler {
	return &BudgetHandler{repo: repo, counter: counter, auditLog: auditLog}
}

// fillSpent enriches a response with the current period spend from the counter.
func (h *BudgetHandler) fillSpent(c *gin.Context, resp *dto.BudgetResponse, b *budget.Budget) {
	if h.counter == nil {
		return
	}
	spent, err := h.counter.Current(c.Request.Context(), b.CounterKey(time.Now().UTC()))
	if err != nil {
		return
	}
	resp.Spent = spent
}

// @sk-task 301-budget-enforcement#T3.1: Create persists a new budget (AC-006)
func (h *BudgetHandler) Create(c *gin.Context) {
	var req dto.CreateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}

	id, err := budget.NewBudgetID()
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to generate budget id")
		return
	}
	b, err := budget.NewBudget(id, req.TenantID, budget.Scope(req.Scope), budget.PeriodType(req.Type))
	if err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}
	b.VirtualKeyID = req.VirtualKeyID
	b.Model = req.Model
	b.CustomDays = req.CustomDays
	b.SoftLimit = req.SoftLimit
	b.HardLimit = req.HardLimit
	if req.Currency != "" {
		b.Currency = req.Currency
	}
	b.NotifyAt = req.NotifyAt
	if b.NotifyAt == nil {
		b.NotifyAt = []float64{}
	}

	if err := h.repo.Create(c.Request.Context(), b); err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to create budget")
		return
	}

	h.writeBudgetAudit(c, "create_budget", b.ID, map[string]any{
		"tenant_id": b.TenantID, "scope": string(b.Scope), "type": string(b.Type),
	})
	resp := dto.BudgetToResponse(b)
	h.fillSpent(c, &resp, b)
	c.JSON(http.StatusCreated, resp)
}

// @sk-task 301-budget-enforcement#T3.1: List returns all budgets (AC-006)
func (h *BudgetHandler) List(c *gin.Context) {
	q := parseListQuery(c)

	var (
		budgets []*budget.Budget
		total   int
		err     error
	)
	if q.Active {
		budgets, total, err = h.repo.ListPaged(c.Request.Context(), q.Limit, q.Offset, q.Search)
	} else {
		budgets, err = h.repo.List(c.Request.Context())
		total = len(budgets)
	}
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to list budgets")
		return
	}
	out := make([]dto.BudgetResponse, len(budgets))
	for i, b := range budgets {
		resp := dto.BudgetToResponse(b)
		h.fillSpent(c, &resp, b)
		out[i] = resp
	}
	if q.Active {
		writePage(c, out, q, total)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// @sk-task 301-budget-enforcement#T3.1: ListByTenant returns budgets of a tenant (AC-006)
func (h *BudgetHandler) ListByTenant(c *gin.Context) {
	budgets, err := h.repo.ListByTenant(c.Request.Context(), c.Param("slug"))
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to list budgets")
		return
	}
	out := make([]dto.BudgetResponse, len(budgets))
	for i, b := range budgets {
		resp := dto.BudgetToResponse(b)
		h.fillSpent(c, &resp, b)
		out[i] = resp
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// @sk-task 301-budget-enforcement#T3.1: Get returns a single budget (AC-006)
func (h *BudgetHandler) Get(c *gin.Context) {
	b, err := h.repo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, budget.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "budget not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to get budget")
		return
	}
	resp := dto.BudgetToResponse(b)
	h.fillSpent(c, &resp, b)
	c.JSON(http.StatusOK, resp)
}

// @sk-task 301-budget-enforcement#T3.1: Update modifies an existing budget (AC-006)
func (h *BudgetHandler) Update(c *gin.Context) {
	var req dto.UpdateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, err.Error())
		return
	}

	b, err := h.repo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, budget.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "budget not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to get budget")
		return
	}

	if req.Scope != nil {
		b.Scope = budget.Scope(*req.Scope)
	}
	if req.Type != nil {
		b.Type = budget.PeriodType(*req.Type)
	}
	if req.VirtualKeyID != nil {
		b.VirtualKeyID = *req.VirtualKeyID
	}
	if req.Model != nil {
		b.Model = *req.Model
	}
	if req.CustomDays != nil {
		b.CustomDays = *req.CustomDays
	}
	if req.SoftLimit != nil {
		b.SoftLimit = req.SoftLimit
	}
	if req.HardLimit != nil {
		b.HardLimit = req.HardLimit
	}
	if req.Currency != nil {
		b.Currency = *req.Currency
	}
	if req.NotifyAt != nil {
		b.NotifyAt = req.NotifyAt
	}
	if req.Enabled != nil {
		b.Enabled = *req.Enabled
	}

	if err := h.repo.Update(c.Request.Context(), b); err != nil {
		if errors.Is(err, budget.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "budget not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to update budget")
		return
	}

	h.writeBudgetAudit(c, "update_budget", b.ID, map[string]any{"tenant_id": b.TenantID})
	resp := dto.BudgetToResponse(b)
	h.fillSpent(c, &resp, b)
	c.JSON(http.StatusOK, resp)
}

// @sk-task 301-budget-enforcement#T3.1: Delete removes a budget (AC-006)
func (h *BudgetHandler) Delete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, budget.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "budget not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to delete budget")
		return
	}
	h.writeBudgetAudit(c, "delete_budget", c.Param("id"), nil)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// @sk-task 301-budget-enforcement#T3.1: History returns spend entries for a budget (AC-006)
func (h *BudgetHandler) History(c *gin.Context) {
	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	entries, err := h.repo.History(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		if errors.Is(err, budget.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "budget not found")
			return
		}
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to load budget history")
		return
	}
	out := make([]dto.SpendEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = dto.SpendEntryToResponse(e)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *BudgetHandler) writeBudgetAudit(c *gin.Context, action, target string, details map[string]any) {
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
