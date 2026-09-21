package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-test 10-gateway-skeleton#T4.1: TestHealthEndpoint (AC-001)
func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	srv.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"status":"ok"}` {
		t.Errorf("expected body {\"status\":\"ok\"}, got %s", w.Body.String())
	}
}

// @sk-test 10-gateway-skeleton#T4.1: TestReadyEndpoint (AC-002)
func TestReadyEndpoint(t *testing.T) {
	srv := newTestServer()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	srv.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 10-gateway-skeleton#T4.1: TestLiveEndpoint (AC-003)
func TestLiveEndpoint(t *testing.T) {
	srv := newTestServer()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/live", nil)
	srv.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"status":"ok"}` {
		t.Errorf("expected body {\"status\":\"ok\"}, got %s", w.Body.String())
	}
}

// @sk-test 10-gateway-skeleton#T4.1: TestRequestIDHeader (AC-004)
func TestRequestIDHeader(t *testing.T) {
	srv := newTestServer()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	srv.engine.ServeHTTP(w, req)

	rid := w.Header().Get("X-Request-ID")
	if rid == "" {
		t.Error("expected X-Request-ID header, got empty")
	}
}

// @sk-test 10-gateway-skeleton#T4.1: TestPanicRecovery (AC-006)
func TestPanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine.Use(middleware.Recovery(log))
	engine.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on second request, got %d", w2.Code)
	}
}

// @sk-test 117-critical-test-coverage#T2.1: TestGracefulShutdown (AC-001)
func TestGracefulShutdown(t *testing.T) {
	srv := newTestServer()
	srv.engine.GET("/slow", func(c *gin.Context) {
		time.Sleep(200 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	srv.HTTP = &http.Server{Handler: srv.engine}
	go srv.HTTP.Serve(listener)
	defer srv.HTTP.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/slow", port))
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}

	select {
	case <-errCh:
	case <-time.After(1 * time.Second):
		t.Error("request did not complete after shutdown")
	}
}

// @sk-test 117-critical-test-coverage#T3.5: TestNotFoundRoute (AC-001)
func TestNotFoundRoute(t *testing.T) {
	srv := newTestServer()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/nonexistent", nil)
	srv.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// @sk-test 117-critical-test-coverage#T3.5: TestMetricsRoute (AC-001)
func TestMetricsRoute(t *testing.T) {
	srv := newTestServer()
	var called bool
	srv.RegisterMetricsRoute(func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	srv.engine.ServeHTTP(w, req)

	if !called {
		t.Error("expected metrics handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test anthropic-messages-endpoint#T4.1: TestMessagesEndpointRegistered — POST /api/v1/messages returns 200 (AC-001)
func TestMessagesEndpointRegistered(t *testing.T) {
	srv := newTestServer()

	srv.RegisterProxyRoute(nil, nil)

	w := httptest.NewRecorder()
	body := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.engine.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("expected /api/v1/messages to be registered, got 404")
	}
}

// @sk-test anthropic-messages-endpoint#T4.1: TestMessagesAliasFromV1 — /v1/messages is served directly (AC-001)
func TestMessagesAliasFromV1(t *testing.T) {
	srv := newTestServer()

	srv.RegisterProxyRoute(nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"m","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	srv.engine.ServeHTTP(w, req)

	// The /v1 alias must be registered (no redirect, no 404).
	if w.Code == http.StatusNotFound || w.Code == http.StatusPermanentRedirect || w.Code == http.StatusMovedPermanently {
		t.Errorf("expected /v1/messages to be served, got %d", w.Code)
	}
}

// newCompletionsServer builds a gateway server whose /completions chain is fully
// wired: virtual-key auth + model access + caller-provided usage/budget stages.
func newCompletionsServer(t *testing.T, usageMw, budgetMw gin.HandlerFunc) (*Server, func(path, model string) *httptest.ResponseRecorder) {
	t.Helper()
	repo := &selfFakeRepo{keys: []*virtualkey.VirtualKey{{
		ID:            "k1",
		TenantID:      "acme",
		KeyHash:       virtualkey.KeyHash("sk-mc_secret"),
		AllowedModels: []string{"gpt-4o-mini"},
		Enabled:       true,
	}}}
	slug, err := value.NewTenantSlug("acme")
	if err != nil {
		t.Fatalf("slug: %v", err)
	}
	tenant := entity.NewTenant(slug, "Acme", "Authorization")

	srv := newTestServer()
	srv.RegisterAuth(middleware.VirtualKeyAuth(repo, middleware.NewTenantProvider([]*entity.Tenant{tenant})))
	srv.RegisterModelAccess(middleware.ModelAccess())
	if usageMw != nil {
		srv.RegisterUsageMiddleware(usageMw)
	}
	if budgetMw != nil {
		srv.RegisterBudgetMiddleware(budgetMw)
	}
	srv.RegisterProxyRoute(func(c *gin.Context) { c.Next() }, nil)

	post := func(path, model string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		body := fmt.Sprintf(`{"model":%q,"prompt":"hi"}`, model)
		req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer sk-mc_secret")
		srv.engine.ServeHTTP(w, req)
		return w
	}
	return srv, post
}

// @sk-test usage-accounting-integrity#T4.2: /completions runs model-access, usage and budget (AC-005)
func TestCompletionsUsesFullChain(t *testing.T) {
	usageRan, budgetRan := 0, 0
	_, post := newCompletionsServer(t,
		func(c *gin.Context) { usageRan++; c.Next() },
		func(c *gin.Context) { budgetRan++; c.Next() },
	)

	for _, path := range []string{"/api/v1/completions", "/v1/completions"} {
		if w := post(path, "gpt-4o"); w.Code != http.StatusForbidden {
			t.Errorf("%s disallowed model: expected 403, got %d: %s", path, w.Code, w.Body.String())
		}
		if w := post(path, "gpt-4o-mini"); w.Code != http.StatusOK {
			t.Errorf("%s allowed model: expected 200, got %d: %s", path, w.Code, w.Body.String())
		}
	}
	if usageRan == 0 || budgetRan == 0 {
		t.Errorf("expected usage and budget middleware to run on /completions (usage=%d budget=%d)", usageRan, budgetRan)
	}
}

// @sk-test usage-accounting-integrity#T4.2: budget stage can block /completions with 429 (AC-005)
func TestCompletionsBudgetStageReturns429(t *testing.T) {
	_, post := newCompletionsServer(t, nil, func(c *gin.Context) {
		middleware.AbortWithError(c, http.StatusTooManyRequests, middleware.ErrorCodeBudgetExceeded, "budget exceeded")
	})

	for _, path := range []string{"/api/v1/completions", "/v1/completions"} {
		w := post(path, "gpt-4o-mini")
		if w.Code != http.StatusTooManyRequests {
			t.Errorf("%s: expected 429, got %d: %s", path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "BUDGET_EXCEEDED") {
			t.Errorf("%s: expected BUDGET_EXCEEDED code, got %s", path, w.Body.String())
		}
	}
}

// @sk-test 117-critical-test-coverage#T3.5: TestNilRoutingHandler (AC-001)
func TestNilRoutingHandler(t *testing.T) {
	srv := newTestServer()

	srv.RegisterProxyRoute(nil, nil)

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from legacy handler, got %d: %s", w.Code, w.Body.String())
	}
}

func newTestServer() *Server {
	gin.SetMode(gin.TestMode)
	cfg := &config.ServerConfig{
		Port:        0,
		HealthCheck: &config.HealthCheckConfig{CriticalDeps: []string{"database"}},
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	return New(cfg, log, "", health.NewService(nil))
}
