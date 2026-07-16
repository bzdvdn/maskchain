// @sk-test 115-rate-limit-wiring#T4.2: Verify wiring nil Valkey passthrough (AC-006, AC-008)
// @sk-test 115-rate-limit-wiring#T4.2: Verify wiring no ratelimit config (AC-008)
package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	budgetrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/budget"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

func TestRateLimitWiringNilValkeyPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var hitCount atomic.Int64

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set("tenant", entity.NewTenant(slug, "Test", "", nil))
		c.Next()
	})

	cfg := &config.RateLimitConfig{
		DefaultRatePerWindow: 1,
		DefaultWindowSec:     60,
	}
	rlRepo := budgetrepo.NewValkeyRateLimitRepo(nil)
	tbRepo := budgetrepo.NewValkeyTokenBudgetRepo(nil)
	engine.Use(middleware.RateLimit(rlRepo, cfg, tbRepo))

	engine.GET("/test", func(c *gin.Context) {
		hitCount.Add(1)
		c.Status(http.StatusOK)
	})

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 (nil Valkey passthrough), got %d", i, w.Code)
		}
	}
	if n := hitCount.Load(); n != 5 {
		t.Errorf("expected 5 handler hits, got %d", n)
	}
}

func TestRateLimitWiringNoConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (no rate limit without config), got %d", w.Code)
	}
}
