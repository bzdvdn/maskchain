package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

func testNow() time.Time { return time.Now().UTC() }

func budgetMiddlewareTestHandler(b *budget.Budget, counter *fakeBudgetCounter, respBody string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	slug, _ := value.NewTenantSlug("alpha")
	tenant := entity.NewTenant(slug, "Alpha", "X-Mask-Auth", entity.WithTenantDictionaries(nil))

	repo := &fakeBudgetRepo{budgets: []*budget.Budget{b}}
	if counter == nil {
		counter = &fakeBudgetCounter{values: map[string]float64{}}
	}
	notifier := &fakeBudgetNotifier{}
	rates := analytics.NewCostRateRegistry([]*analytics.CostRate{
		{Model: "gpt-4", InputPricePer1K: 10, OutputPricePer1K: 30, Currency: "USD"},
	})

	mw := NewBudgetMiddleware(repo, counter, rates, nil, notifier, slog.Default())

	engine.Use(func(c *gin.Context) {
		c.Set(tenantKey, tenant)
		c.Set(virtualKeyContextKey, fakeKey("h1", "alpha", nil, nil))
		c.Next()
	})
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(respBody))
	})

	req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4"}`))
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test 301-budget-enforcement#T2.3: Under hard limit the request passes (AC-002)
func TestBudgetMiddlewareUnderLimitPasses(t *testing.T) {
	hard := 100.0
	b, _ := budget.NewBudget("b1", "alpha", budget.ScopeTenant, budget.PeriodMonthly)
	b.HardLimit = &hard

	counter := &fakeBudgetCounter{values: map[string]float64{b.CounterKey(testNow()): 10}}
	w := budgetMiddlewareTestHandler(b, counter, `{"usage":{"prompt_tokens":10,"completion_tokens":10}}`)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 301-budget-enforcement#T2.3: At hard limit the request is rejected with 429 (AC-002)
func TestBudgetMiddlewareHardLimitRejects(t *testing.T) {
	hard := 100.0
	b, _ := budget.NewBudget("b1", "alpha", budget.ScopeTenant, budget.PeriodMonthly)
	b.HardLimit = &hard

	counter := &fakeBudgetCounter{values: map[string]float64{b.CounterKey(testNow()): 100}}
	w := budgetMiddlewareTestHandler(b, counter, `{"usage":{"prompt_tokens":10,"completion_tokens":10}}`)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

// @sk-test 301-budget-enforcement#T2.3: Successful request increments the counter (AC-002)
func TestBudgetMiddlewareIncrementsCounter(t *testing.T) {
	hard := 100.0
	b, _ := budget.NewBudget("b1", "alpha", budget.ScopeTenant, budget.PeriodMonthly)
	b.HardLimit = &hard

	counter := &fakeBudgetCounter{values: map[string]float64{}}
	w := budgetMiddlewareTestHandler(b, counter, `{"usage":{"prompt_tokens":1000,"completion_tokens":1000}}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	spent := counter.values[b.CounterKey(testNow())]
	// 1000 in @ 10/1k + 1000 out @ 30/1k = 10 + 30 = 40
	if spent != 40 {
		t.Errorf("expected counter 40, got %f", spent)
	}
}

// @sk-test 301-budget-enforcement#T2.3: Crossing notify_at fires an alert (AC-003)
func TestBudgetMiddlewareFiresAlertOnThreshold(t *testing.T) {
	hard := 100.0
	b, _ := budget.NewBudget("b1", "alpha", budget.ScopeTenant, budget.PeriodMonthly)
	b.HardLimit = &hard
	b.NotifyAt = []float64{50}

	repo := &fakeBudgetRepo{budgets: []*budget.Budget{b}}
	counter := &fakeBudgetCounter{values: map[string]float64{}}
	notifier := &fakeBudgetNotifier{}
	rates := analytics.NewCostRateRegistry([]*analytics.CostRate{
		{Model: "gpt-4", InputPricePer1K: 10, OutputPricePer1K: 30, Currency: "USD"},
	})

	mw := NewBudgetMiddleware(repo, counter, rates, nil, notifier, slog.Default())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	slug, _ := value.NewTenantSlug("alpha")
	tenant := entity.NewTenant(slug, "Alpha", "X-Mask-Auth", entity.WithTenantDictionaries(nil))

	engine.Use(func(c *gin.Context) {
		c.Set(tenantKey, tenant)
		c.Next()
	})
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(`{"usage":{"prompt_tokens":0,"completion_tokens":5000}}`))
	})

	req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4"}`))
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(notifier.alerts) != 1 || notifier.alerts[0] != 50 {
		t.Errorf("expected one alert at 50%%, got %v", notifier.alerts)
	}
}

// @sk-test 301-budget-enforcement#T2.3: No matching budgets passes through (AC-002)
func TestBudgetMiddlewareNoBudgetPasses(t *testing.T) {
	b, _ := budget.NewBudget("b1", "other-tenant", budget.ScopeTenant, budget.PeriodMonthly)
	hard := 1.0
	b.HardLimit = &hard

	w := budgetMiddlewareTestHandler(b, nil, `{"usage":{"prompt_tokens":10,"completion_tokens":10}}`)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
