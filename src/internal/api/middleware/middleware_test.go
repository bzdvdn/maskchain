package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// @sk-test 10-gateway-skeleton#T4.2: TestRequestID generates UUID (AC-004)
func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	rid := w.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("expected X-Request-ID header")
	}
	if !strings.Contains(rid, "-") {
		t.Errorf("expected UUID format, got %q", rid)
	}
}

// @sk-test 10-gateway-skeleton#T4.2: TestRequestID preserves existing header (AC-004)
func TestRequestID_PreservesExisting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	engine.ServeHTTP(w, req)

	if rid := w.Header().Get("X-Request-ID"); rid != "existing-id" {
		t.Errorf("expected existing-id, got %q", rid)
	}
}

// @sk-test 10-gateway-skeleton#T4.2: TestRecovery catches panic (AC-006)
func TestRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	log, _ := zap.NewProduction()
	engine.Use(Recovery(log))
	engine.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// @sk-test 10-gateway-skeleton#T4.2: TestLogger writes all fields (AC-007)
func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, recorded := observer.New(zapcore.InfoLevel)
	log := zap.New(core)

	engine := gin.New()
	engine.Use(RequestID())
	engine.Use(Logger(log))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if recorded.Len() == 0 {
		t.Fatal("expected log entry")
	}
	entry := recorded.All()[0]

	var method, path, rid string
	var status int
	var hasDuration bool
	for _, f := range entry.Context {
		switch f.Key {
		case "method":
			method = f.String
		case "path":
			path = f.String
		case "status":
			status = int(f.Integer)
		case "duration":
			hasDuration = true
		case "request_id":
			rid = f.String
		}
	}

	if method != "GET" {
		t.Errorf("expected method GET, got %q", method)
	}
	if path != "/test" {
		t.Errorf("expected path /test, got %q", path)
	}
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}
	if !hasDuration {
		t.Error("expected duration field")
	}
	if rid == "" {
		t.Error("expected request_id field")
	}
}

// @sk-test 10-gateway-skeleton#T4.2: TestCORS allows configured origin (AC-008)
func TestCORS_AllowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORS([]string{"http://example.com"}))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	engine.ServeHTTP(w, req)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://example.com" {
		t.Errorf("expected http://example.com, got %q", origin)
	}
}

// @sk-test 10-gateway-skeleton#T4.2: TestCORS blocks non-configured origin (AC-008)
func TestCORS_BlockedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORS([]string{"http://example.com"}))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	engine.ServeHTTP(w, req)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected no CORS header, got %q", origin)
	}
}

// @sk-test 10-gateway-skeleton#T4.2: TestCORS handles wildcard (AC-008)
func TestCORS_Wildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORS([]string{"*"}))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	engine.ServeHTTP(w, req)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected *, got %q", origin)
	}
}

// @sk-test 10-gateway-skeleton#T4.2: TestCORS handles preflight (AC-008)
func TestCORS_Preflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORS([]string{"http://example.com"}))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", w.Code)
	}
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://example.com" {
		t.Errorf("expected http://example.com, got %q", origin)
	}
}
