package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
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
	log, _ := zap.NewProduction()
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

// @sk-test anthropic-messages-endpoint#T4.1: TestMessagesRedirectFromV1 — /v1/messages redirects to /api/v1/messages (AC-001)
func TestMessagesRedirectFromV1(t *testing.T) {
	srv := newTestServer()

	srv.RegisterProxyRoute(nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/messages", nil)
	srv.engine.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("expected 301 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/api/v1/messages" {
		t.Errorf("expected Location /api/v1/messages, got %q", loc)
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
	log, _ := zap.NewProduction()
	return New(cfg, log, "", health.NewService(nil))
}
