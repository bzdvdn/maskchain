package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

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
	if w.Body.String() != `{"status":"ok"}` {
		t.Errorf("expected body {\"status\":\"ok\"}, got %s", w.Body.String())
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
	if w.Body.String() != `{"status":"alive"}` {
		t.Errorf("expected body {\"status\":\"alive\"}, got %s", w.Body.String())
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

	// server still works after panic
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on second request, got %d", w2.Code)
	}
}

func newTestServer() *Server {
	gin.SetMode(gin.TestMode)
	cfg := &config.ServerConfig{Port: 0}
	log, _ := zap.NewProduction()
	return New(cfg, log)
}
