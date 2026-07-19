package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

func testCostRates() []*analytics.CostRate {
	return []*analytics.CostRate{
		{Model: "gpt-4", InputPricePer1K: 0.01, OutputPricePer1K: 0.03},
		{Model: "gpt-3.5-turbo", InputPricePer1K: 0.001, OutputPricePer1K: 0.002},
	}
}

func newTestUsageMiddleware(t *testing.T) (*UsageMiddleware, *testRecordHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	metrics.Reset()
	reg := analytics.NewCostRateRegistry(testCostRates())
	usageCh := make(chan analytics.TokenUsage, 100)
	rec, log := newTestLogger(t)
	rec.level = slog.LevelWarn
	return NewUsageMiddleware(reg, usageCh, log), rec
}

func usageResponseBody(promptTokens, completionTokens int64) string {
	resp := map[string]interface{}{
		"id":      "chatcmpl-123",
		"object":  "chat.completion",
		"model":   "gpt-4",
		"choices": []map[string]interface{}{{"message": map[string]interface{}{"role": "assistant", "content": "Hello!"}}},
		"usage": map[string]int64{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func noUsageResponseBody() string {
	resp := map[string]interface{}{
		"id":      "chatcmpl-123",
		"object":  "chat.completion",
		"choices": []map[string]interface{}{{"message": map[string]interface{}{"role": "assistant", "content": "Hello!"}}},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// @sk-test 131-analytics-pipeline#T4.1: TestUsageMiddlewareParsing (AC-001)
func TestUsageMiddlewareParsing(t *testing.T) {
	mw, _ := newTestUsageMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(usageResponseBody(100, 50)))
	})

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	inputVal := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues("unknown", "gpt-4", "input"))
	if inputVal != 100 {
		t.Errorf("expected input tokens 100, got %f", inputVal)
	}
	outputVal := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues("unknown", "gpt-4", "output"))
	if outputVal != 50 {
		t.Errorf("expected output tokens 50, got %f", outputVal)
	}
	reqVal := testutil.ToFloat64(metrics.RequestTotal.WithLabelValues("unknown", "gpt-4"))
	if reqVal != 1 {
		t.Errorf("expected request count 1, got %f", reqVal)
	}
}

// @sk-test 131-analytics-pipeline#T4.1: TestUsageMetricsUpdate (AC-003)
func TestUsageMetricsUpdate(t *testing.T) {
	mw, _ := newTestUsageMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(usageResponseBody(200, 100)))
	})

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	inputVal := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues("unknown", "gpt-4", "input"))
	if inputVal != 200 {
		t.Errorf("input tokens: expected 200, got %f", inputVal)
	}
	outputVal := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues("unknown", "gpt-4", "output"))
	if outputVal != 100 {
		t.Errorf("output tokens: expected 100, got %f", outputVal)
	}
	reqVal := testutil.ToFloat64(metrics.RequestTotal.WithLabelValues("unknown", "gpt-4"))
	if reqVal != 1 {
		t.Errorf("request count: expected 1, got %f", reqVal)
	}
}

// @sk-test 131-analytics-pipeline#T4.1: TestUsageMiddlewareNoUsage (AC-005)
func TestUsageMiddlewareNoUsage(t *testing.T) {
	mw, recorded := newTestUsageMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(noUsageResponseBody()))
	})

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var found bool
	for _, entry := range recorded.All() {
		if entry.Level == slog.LevelWarn && strings.Contains(entry.Message, "no usage field") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning log about missing usage field")
	}
}

// @sk-test 131-analytics-pipeline#T4.1: TestUsageMiddlewareStreamingSkip (AC-001, AC-005)
func TestUsageMiddlewareStreamingSkip(t *testing.T) {
	mw, _ := newTestUsageMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(usageResponseBody(100, 50)))
	})

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	inputVal := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues("unknown", "gpt-4", "input"))
	if inputVal != 0 {
		t.Errorf("expected no token metrics for streaming, got %f", inputVal)
	}
}

// @sk-test 131-analytics-pipeline#T4.1: TestUsageMiddlewareNonPost (AC-001)
func TestUsageMiddlewareNonPost(t *testing.T) {
	mw, _ := newTestUsageMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.GET("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(usageResponseBody(100, 50)))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/completions", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	inputVal := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues("unknown", "gpt-4", "input"))
	if inputVal != 0 {
		t.Errorf("expected no token metrics for GET, got %f", inputVal)
	}
}

// @sk-test 131-analytics-pipeline#T4.1: TestUsageMiddlewareCostComputation (AC-001, AC-007)
func TestUsageMiddlewareCostComputation(t *testing.T) {
	usageCh := make(chan analytics.TokenUsage, 10)
	reg := analytics.NewCostRateRegistry(testCostRates())
	_, mwLog := newTestLogger(t)
	mw := NewUsageMiddleware(reg, usageCh, mwLog)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(usageResponseBody(1000, 500)))
	})

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	select {
	case usage := <-usageCh:
		expectedCost := 1.0*0.01 + 0.5*0.03
		if usage.Cost != expectedCost {
			t.Errorf("Cost = %f, want %f", usage.Cost, expectedCost)
		}
		if usage.InputTokens != 1000 {
			t.Errorf("InputTokens = %d, want %d", usage.InputTokens, 1000)
		}
		if usage.OutputTokens != 500 {
			t.Errorf("OutputTokens = %d, want %d", usage.OutputTokens, 500)
		}
	default:
		t.Error("expected TokenUsage in channel")
	}
}
