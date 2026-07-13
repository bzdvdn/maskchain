package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

type mockRateLimitRepo struct {
	allowFunc func(ctx context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error)
}

func (m *mockRateLimitRepo) Allow(ctx context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
	return m.allowFunc(ctx, key, limit, windowSec)
}

func (m *mockRateLimitRepo) Reset(ctx context.Context, key string) error {
	return nil
}

type mockTokenBudgetRepo struct {
	remainingFunc func(ctx context.Context, key string, limit int64) (int64, error)
	deductFunc    func(ctx context.Context, key string, tokens int64, ttlSec int64) (int64, error)
}

func (m *mockTokenBudgetRepo) Remaining(ctx context.Context, key string, limit int64) (int64, error) {
	return m.remainingFunc(ctx, key, limit)
}

func (m *mockTokenBudgetRepo) Deduct(ctx context.Context, key string, tokens int64, ttlSec int64) (int64, error) {
	return m.deductFunc(ctx, key, tokens, ttlSec)
}

func (m *mockTokenBudgetRepo) Reset(ctx context.Context, key string) error {
	return nil
}

// @sk-test rate-limiting-budgets#T2.3: TestRateLimitAllowsWithinLimit (AC-001)
func TestRateLimitAllowsWithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{
				Allowed:   true,
				Limit:     limit,
				Remaining: limit - 1,
				ResetTime: time.Now().Add(time.Duration(windowSec) * time.Second).UnixMilli(),
			}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 10,
		DefaultWindowSec:     60,
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test rate-limiting-budgets#T2.3: TestRateLimitBlocksWhenExceeded (AC-001)
func TestRateLimitBlocksWhenExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{
				Allowed:   false,
				Limit:     limit,
				Remaining: 0,
				ResetTime: time.Now().Add(time.Duration(windowSec) * time.Second).UnixMilli(),
			}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 10,
		DefaultWindowSec:     60,
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "rate_limit_exceeded" {
		t.Errorf("expected error 'rate_limit_exceeded', got %q", body["error"])
	}
}

// @sk-test rate-limiting-budgets#T2.3: TestRateLimitSkipsWithoutTenant (AC-001)
func TestRateLimitSkipsWithoutTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{
				Allowed:   false,
				Limit:     limit,
				Remaining: 0,
				ResetTime: time.Now().UnixMilli(),
			}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 10,
		DefaultWindowSec:     60,
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (skip without tenant), got %d", w.Code)
	}
}

// @sk-test rate-limiting-budgets#T2.3: TestRateLimitRecoversAfterWindow (AC-004)
func TestRateLimitRecoversAfterWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var callCount int
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			callCount++
			if callCount <= 1 {
				return &budget.RateLimit{Allowed: false, Limit: limit, Remaining: 0, ResetTime: 0}, nil
			}
			return &budget.RateLimit{Allowed: true, Limit: limit, Remaining: limit - 1, ResetTime: 0}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 10,
		DefaultWindowSec:     60,
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)

	engine.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 on first call, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 after window recovery, got %d", w2.Code)
	}
}

// @sk-test rate-limiting-budgets#T3.4: TestRateLimitHeadersOnSuccess checks headers on 200 (AC-003)
func TestRateLimitHeadersOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{
				Allowed:   true,
				Limit:     limit,
				Remaining: limit - 1,
				ResetTime: time.Now().Add(time.Duration(windowSec) * time.Second).UnixMilli(),
			}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 10,
		DefaultWindowSec:     60,
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("expected X-RateLimit-Remaining header")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("expected X-RateLimit-Reset header")
	}
}

// @sk-test rate-limiting-budgets#T3.4: TestRateLimitHeadersOn429 checks headers on 429 (AC-003)
func TestRateLimitHeadersOn429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{
				Allowed:   false,
				Limit:     limit,
				Remaining: 0,
				ResetTime: time.Now().Add(time.Duration(windowSec) * time.Second).UnixMilli(),
			}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 10,
		DefaultWindowSec:     60,
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header on 429")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("expected X-RateLimit-Remaining header on 429")
	}
}

// @sk-test rate-limiting-budgets#T3.4: TestRateLimitPerTenantConfig tests different limits per tenant (AC-006)
func TestRateLimitPerTenantConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var localCallCount int
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slugA, _ := value.NewTenantSlug("tenant-a")
		c.Set(tenantKey, entity.NewTenant(slugA, "Tenant-A", "", nil))
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			localCallCount++
			allowed := localCallCount <= int(limit)
			remaining := limit - int64(localCallCount)
			if remaining < 0 {
				remaining = 0
			}
			return &budget.RateLimit{Allowed: allowed, Limit: limit, Remaining: remaining, ResetTime: 0}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 100,
		DefaultWindowSec:     60,
		TenantOverrides: map[string]*config.RateLimitOverride{
			"tenant-a": {RatePerWindow: intPtr(10)},
		},
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)

	for i := 0; i < 15; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if i < 10 && w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200 for tenant-a, got %d", i, w.Code)
		}
		if i >= 10 && w.Code != http.StatusTooManyRequests {
			t.Errorf("request %d: expected 429 for tenant-a, got %d", i, w.Code)
		}
	}
}

// @sk-test rate-limiting-budgets#T3.4: TestRateLimitMetrics verifies Prometheus counter (AC-007)
func TestRateLimitMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reg := prometheus.NewRegistry()
	metrics.RegisterMetrics(reg)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{Allowed: false, Limit: limit, Remaining: 0, ResetTime: 0}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 10,
		DefaultWindowSec:     60,
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	metricsHandler := httptest.NewRecorder()
	metricsReq, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	handler.ServeHTTP(metricsHandler, metricsReq)

	if !strings.Contains(metricsHandler.Body.String(), `rate_limited_total`) {
		t.Error("expected rate_limited_total metric in output")
	}
}

// @sk-test rate-limiting-budgets#T3.6: TestTokenBudgetBlocksWhenExceeded (AC-002)
func TestTokenBudgetBlocksWhenExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Set("model", "gpt-4")
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{Allowed: true, Limit: limit, Remaining: limit - 1, ResetTime: 0}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 100,
		DefaultWindowSec:     60,
		DefaultTokenBudget:   map[string]int64{"gpt-4": 1000},
	}, &mockTokenBudgetRepo{
		remainingFunc: func(_ context.Context, key string, limit int64) (int64, error) {
			return 0, nil
		},
	}))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 (token budget exceeded), got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "token_budget_exceeded" {
		t.Errorf("expected error 'token_budget_exceeded', got %q", body["error"])
	}
}

// @sk-test rate-limiting-budgets#T3.6: TestTokenBudgetPerModel tests model isolation (AC-005)
func TestTokenBudgetPerModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		if strings.Contains(c.Request.URL.Path, "gpt4") {
			c.Set("model", "gpt-4")
		} else {
			c.Set("model", "gpt-3.5-turbo")
		}
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{Allowed: true, Limit: limit, Remaining: limit - 1, ResetTime: 0}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 100,
		DefaultWindowSec:     60,
		DefaultTokenBudget:   map[string]int64{"gpt-4": 1000, "gpt-3.5-turbo": 5000},
	}, &mockTokenBudgetRepo{
		remainingFunc: func(_ context.Context, key string, limit int64) (int64, error) {
			if strings.Contains(key, "gpt-4") {
				return 0, nil
			}
			return limit, nil
		},
	}))
	engine.GET("/test-gpt4", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	engine.GET("/test-gpt35", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test-gpt4", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for gpt-4 (budget exceeded), got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/test-gpt35", nil)
	engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 for gpt-3.5-turbo (budget available), got %d", w2.Code)
	}
}

func intPtr(i int) *int {
	return &i
}

// @sk-test rate-limiting-budgets#T3.6: TestTokenBudgetHeader checks X-RateLimit-Budget-Remaining (AC-003)
func TestTokenBudgetHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Set("model", "gpt-4")
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			return &budget.RateLimit{Allowed: true, Limit: limit, Remaining: limit - 1, ResetTime: 0}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 100,
		DefaultWindowSec:     60,
		DefaultTokenBudget:   map[string]int64{"gpt-4": 1000},
	}, &mockTokenBudgetRepo{
		remainingFunc: func(_ context.Context, key string, limit int64) (int64, error) {
			return 500, nil
		},
	}))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Header().Get("X-RateLimit-Budget-Remaining") != "500" {
		t.Errorf("expected X-RateLimit-Budget-Remaining: 500, got %q", w.Header().Get("X-RateLimit-Budget-Remaining"))
	}
}

// @sk-test rate-limiting-budgets#T4.1: TestRateLimitE2E full pipeline with multi-tenant, headers, metrics (AC-all)
func TestRateLimitE2E(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reg := prometheus.NewRegistry()
	metrics.RegisterMetrics(reg)

	var tenantACalls, tenantBCalls int

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		e2eSlug, _ := value.NewTenantSlug(c.Request.Header.Get("e2e_tenant"))
		c.Set(tenantKey, entity.NewTenant(e2eSlug, "E2E", "", nil))
		c.Next()
	})
	engine.Use(RateLimit(&mockRateLimitRepo{
		allowFunc: func(_ context.Context, key string, limit, windowSec int64) (*budget.RateLimit, error) {
			var calls *int
			if key == "ratelimit:tenant-a" {
				calls = &tenantACalls
			} else {
				calls = &tenantBCalls
			}
			*calls++
			allowed := *calls <= int(limit)
			remaining := limit - int64(*calls)
			if remaining < 0 {
				remaining = 0
			}
			return &budget.RateLimit{Allowed: allowed, Limit: limit, Remaining: remaining, ResetTime: 0}, nil
		},
	}, &config.RateLimitConfig{
		DefaultRatePerWindow: 5,
		DefaultWindowSec:     60,
		TenantOverrides: map[string]*config.RateLimitOverride{
			"tenant-b": {RatePerWindow: intPtr(10)},
		},
	}, nil))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)

	for i := 0; i < 8; i++ {
		w := httptest.NewRecorder()
		req.Header.Set("e2e_tenant", "tenant-a")
		engine.ServeHTTP(w, req)

		if i < 5 {
			if w.Code != http.StatusOK {
				t.Errorf("tenant-a req %d: expected 200, got %d", i, w.Code)
			}
			if w.Header().Get("X-RateLimit-Limit") != "5" {
				t.Errorf("tenant-a req %d: expected X-RateLimit-Limit=5, got %s", i, w.Header().Get("X-RateLimit-Limit"))
			}
		} else {
			if w.Code != http.StatusTooManyRequests {
				t.Errorf("tenant-a req %d: expected 429, got %d", i, w.Code)
			}
			if w.Header().Get("X-RateLimit-Remaining") == "" {
				t.Errorf("tenant-a req %d: expected headers on 429", i)
			}
		}
	}

	for i := 0; i < 8; i++ {
		w := httptest.NewRecorder()
		req.Header.Set("e2e_tenant", "tenant-b")
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("tenant-b req %d: expected 200 (limit=10), got %d", i, w.Code)
		}
	}

	metricsHandler := httptest.NewRecorder()
	metricsReq, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	handler.ServeHTTP(metricsHandler, metricsReq)

	body := metricsHandler.Body.String()
	if !strings.Contains(body, `rate_limited_total`) {
		t.Error("expected rate_limited_total metric in E2E output")
	}
	if !strings.Contains(body, `tenant="tenant-a"`) {
		t.Error("expected tenant-a label in metrics")
	}
}
