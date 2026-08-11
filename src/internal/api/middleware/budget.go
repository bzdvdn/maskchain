package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// @sk-task 301-budget-enforcement#T2.1: BudgetMiddleware enforces budget hard limits and records spend (AC-002, AC-004)
//
// BudgetMiddleware checks budgets before the request and increments spend after
// a successful (non-streaming) response.
type BudgetMiddleware struct {
	repo     budget.BudgetRepository
	counter  budget.SpendCounter
	rates    *analytics.CostRateRegistry
	vkRepo   virtualkey.VirtualKeyRepository
	notifier budget.AlertNotifier
	log      *slog.Logger
}

func NewBudgetMiddleware(
	repo budget.BudgetRepository,
	counter budget.SpendCounter,
	rates *analytics.CostRateRegistry,
	vkRepo virtualkey.VirtualKeyRepository,
	notifier budget.AlertNotifier,
	log *slog.Logger,
) *BudgetMiddleware {
	return &BudgetMiddleware{
		repo:     repo,
		counter:  counter,
		rates:    rates,
		vkRepo:   vkRepo,
		notifier: notifier,
		log:      log,
	}
}

// @sk-task 301-budget-enforcement#T2.1: Handler applies budget enforcement (AC-002, AC-004)
func (m *BudgetMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		model := extractModel(c)
		if model == "" {
			c.Next()
			return
		}

		tenantID, keyID := m.requestContext(c)

		matched, err := m.repo.ListActiveForRequest(c.Request.Context(), tenantID, keyID, model)
		if err != nil {
			m.log.WarnContext(c.Request.Context(), "budget middleware: failed to load budgets",
				slog.String("error", err.Error()))
			c.Next()
			return
		}
		if len(matched) == 0 {
			c.Next()
			return
		}

		now := time.Now().UTC()
		for _, b := range matched {
			spent, err := m.counter.Current(c.Request.Context(), b.CounterKey(now))
			if err != nil {
				m.log.WarnContext(c.Request.Context(), "budget middleware: counter read failed",
					slog.String("budget_id", b.ID), slog.String("error", err.Error()))
				continue
			}
			if b.HardExceeded(spent) {
				m.log.InfoContext(c.Request.Context(), "budget middleware: hard limit exceeded",
					slog.String("budget_id", b.ID), slog.Float64("spent", spent))
				AbortWithError(c, http.StatusTooManyRequests, ErrorCodeBudgetExceeded,
					"budget exceeded")
				return
			}
		}

		if isStreaming(c) {
			c.Next()
			return
		}

		w := &usageBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = w
		c.Next()

		if w.status != http.StatusOK {
			return
		}

		cost, tokens, ok := m.extractUsage(w.body.Bytes(), model)
		if !ok || cost <= 0 {
			return
		}

		for _, b := range matched {
			key := b.CounterKey(now)
			prev, curErr := m.counter.Current(c.Request.Context(), key)
			if curErr != nil {
				m.log.WarnContext(c.Request.Context(), "budget middleware: counter read before increment failed",
					slog.String("budget_id", b.ID), slog.String("error", curErr.Error()))
				prev = 0
			}
			newSpent, incErr := m.counter.Increment(c.Request.Context(), key, cost, b.PeriodTTL(now))
			if incErr != nil {
				m.log.WarnContext(c.Request.Context(), "budget middleware: counter increment failed",
					slog.String("budget_id", b.ID), slog.String("error", incErr.Error()))
				continue
			}

			entry := budget.SpendEntry{
				BudgetID:     b.ID,
				VirtualKeyID: keyID,
				TenantID:     tenantID,
				Model:        model,
				Cost:         cost,
				Tokens:       tokens,
				CreatedAt:    time.Now().UTC(),
			}
			if err := m.repo.RecordSpend(c.Request.Context(), entry); err != nil {
				m.log.WarnContext(c.Request.Context(), "budget middleware: record spend failed",
					slog.String("budget_id", b.ID), slog.String("error", err.Error()))
			}

			for _, pct := range b.CrossedThresholds(prev, newSpent) {
				if m.notifier != nil {
					if err := m.notifier.NotifyBudgetExceeded(c.Request.Context(), b, pct, newSpent); err != nil {
						m.log.WarnContext(c.Request.Context(), "budget middleware: alert failed",
							slog.String("budget_id", b.ID), slog.String("error", err.Error()))
					}
				}
			}

			if b.Scope == budget.ScopeKey && b.VirtualKeyID != "" && m.vkRepo != nil {
				m.accumulateKeySpend(c, b.VirtualKeyID, cost)
			}
		}
	}
}

func (m *BudgetMiddleware) requestContext(c *gin.Context) (tenantID, keyID string) {
	if t, ok := TenantFromContext(c); ok {
		tenantID = t.Slug().String()
	}
	if vk, ok := VirtualKeyFromContext(c); ok {
		keyID = vk.ID
	}
	return tenantID, keyID
}

func (m *BudgetMiddleware) extractUsage(body []byte, model string) (cost float64, tokens int64, ok bool) {
	var resp struct {
		Usage *struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, 0, false
	}
	if resp.Usage == nil {
		return 0, 0, false
	}
	tokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
	cost = m.rates.Lookup(model).Cost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	return cost, tokens, true
}

func (m *BudgetMiddleware) accumulateKeySpend(c *gin.Context, keyID string, cost float64) {
	key, err := m.vkRepo.GetById(c.Request.Context(), keyID)
	if err != nil {
		if !errors.Is(err, virtualkey.ErrNotFound) {
			m.log.WarnContext(c.Request.Context(), "budget middleware: load virtual key failed",
				slog.String("key_id", keyID), slog.String("error", err.Error()))
		}
		return
	}
	key.AddSpend(cost)
	if err := m.vkRepo.Update(c.Request.Context(), key); err != nil {
		m.log.WarnContext(c.Request.Context(), "budget middleware: update key spend failed",
			slog.String("key_id", keyID), slog.String("error", err.Error()))
	}
}
