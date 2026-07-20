package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

func setRateLimitHeaders(c *gin.Context, rl *budget.RateLimit) {
	c.Header("X-RateLimit-Limit", strconv.FormatInt(rl.Limit, 10))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(rl.Remaining, 10))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(rl.ResetTime/1000, 10))
}

// @sk-task rate-limiting-budgets#T2.2: Implement rate limit middleware (AC-001, AC-004)
// @sk-task rate-limiting-budgets#T3.1: Add rate-limit headers (AC-003)
// @sk-task rate-limiting-budgets#T3.3: Add Prometheus metrics counters (AC-007)
//
// RateLimit handles the operation.
func RateLimit(repo budget.RateLimitRepository, cfg *config.RateLimitConfig, tokenBudgetRepo budget.TokenBudgetRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantCtx, ok := TenantFromContext(c)
		if !ok {
			c.Next()
			return
		}
		tenantID := tenantCtx.Slug().String()

		limit := int64(cfg.DefaultRatePerWindow)
		windowSec := int64(cfg.DefaultWindowSec)

		if override, exists := cfg.TenantOverrides[tenantID]; exists {
			if override.RatePerWindow != nil {
				limit = int64(*override.RatePerWindow)
			}
			if override.WindowSec != nil {
				windowSec = int64(*override.WindowSec)
			}
		}

		windowKey := budget.KeyPrefixRateLimit + tenantID
		rl, err := repo.Allow(c, windowKey, limit, windowSec)
		if err != nil {
			c.Next()
			return
		}

		model := c.GetString("model")

		if !rl.Allowed {
			setRateLimitHeaders(c, rl)
			// @sk-task 115-rate-limit-wiring#T2.1: Add Retry-After header (AC-003)
			retryAfter := (rl.ResetTime / 1000) - time.Now().Unix()
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			metrics.RateLimitExceededTotal.WithLabelValues(tenantID, "rate_limit_exceeded").Inc()
			AbortWithError(c, http.StatusTooManyRequests, ErrorCodeRateLimitExceeded, "rate_limit_exceeded")
			return
		}

		setRateLimitHeaders(c, rl)

		var budgetLimit int64

		if tokenBudgetRepo != nil && model != "" {
			if tb, ok := cfg.DefaultTokenBudget[model]; ok {
				budgetLimit = tb
			}
			if override, exists := cfg.TenantOverrides[tenantID]; exists {
				if tb, ok := override.TokenBudget[model]; ok {
					budgetLimit = tb
				}
			}
			if budgetLimit > 0 {
				budgetKey := budget.KeyPrefixTokenBudget + tenantID + ":" + model
				remaining, rErr := tokenBudgetRepo.Remaining(c, budgetKey, budgetLimit)
				if rErr == nil {
					c.Header("X-RateLimit-Budget-Remaining", strconv.FormatInt(remaining, 10))
				}
				if remaining <= 0 {
					metrics.RateLimitExceededTotal.WithLabelValues(tenantID, "token_budget_exceeded").Inc()
					AbortWithError(c, http.StatusTooManyRequests, ErrorCodeTokenBudgetExceeded, "token_budget_exceeded")
					return
				}
			}
		}

		c.Set("rate_limit_remaining", rl.Remaining)
		c.Set("rate_limit_reset", rl.ResetTime)
		c.Set("rate_limit_limit", rl.Limit)

		// @sk-task rate-limiting-budgets#T3.5: Wrap handler for post-response token deduction (AC-002, AC-005)
		if tokenBudgetRepo != nil && model != "" && budgetLimit > 0 {
			c.Next()

			if c.Writer.Status() == http.StatusOK {
				tokens := int64(0)
				if usage, ok := c.Get("token_usage"); ok {
					if u, ok := usage.(int64); ok {
						tokens = u
					}
				}
				if tokens > 0 {
					budgetKey := budget.KeyPrefixTokenBudget + tenantID + ":" + model
					_, _ = tokenBudgetRepo.Deduct(c, budgetKey, tokens, int64(windowSec))
				}
			}
			return
		}

		c.Next()
	}
}
