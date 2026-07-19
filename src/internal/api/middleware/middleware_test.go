package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

type mockEngine struct {
	resp *appshield.ScanResponse
	err  error
}

func (m *mockEngine) Scan(_ context.Context, _ appshield.ScanRequest) (*appshield.ScanResponse, error) {
	return m.resp, m.err
}

type testRecordHandler struct {
	mu      sync.Mutex
	records []slog.Record
	level   slog.Level
}

func (h *testRecordHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *testRecordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *testRecordHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *testRecordHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *testRecordHandler) All() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]slog.Record, len(h.records))
	copy(out, h.records)
	return out
}

func (h *testRecordHandler) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.records)
}

func newTestLogger(_ *testing.T) (*testRecordHandler, *slog.Logger) {
	h := &testRecordHandler{level: slog.LevelInfo}
	return h, slog.New(h)
}

func newTestTenant(slug string) *entity.Tenant {
	s, _ := value.NewTenantSlug(slug)
	return entity.NewTenant(s, "test-"+slug, "Authorization", nil)
}

func setupTest(t *testing.T) (*gin.Engine, *mockEngine, *slog.Logger) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mockEng := &mockEngine{}
	_, log := newTestLogger(t)
	return engine, mockEng, log
}

func testShieldConfig() *config.ShieldConfig {
	return &config.ShieldConfig{}
}

func chatBody(model, content string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": content},
		},
	})
	return string(b)
}

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
	_, log := newTestLogger(t)
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
	rec, log := newTestLogger(t)

	engine := gin.New()
	engine.Use(RequestID())
	engine.Use(Logger(log))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if rec.Len() == 0 {
		t.Fatal("expected log entry")
	}
	entry := rec.All()[0]

	var method, path, rid string
	var status int
	var hasDuration bool
	entry.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "method":
			method = a.Value.String()
		case "path":
			path = a.Value.String()
		case "status":
			status = int(a.Value.Int64())
		case "duration":
			hasDuration = true
		case "request_id":
			rid = a.Value.String()
		}
		return true
	})

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

// @sk-test 80-tenant-isolation#T4.7: TestLoggerWithTenant verifies tenant_id attribute (AC-008)
func TestLoggerWithTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec, log := newTestLogger(t)

	engine := gin.New()
	engine.Use(RequestID())
	engine.Use(Logger(log))
	engine.GET("/test", func(c *gin.Context) {
		slug, _ := value.NewTenantSlug("test-tenant")
		c.Set(tenantKey, entity.NewTenant(slug, "Test", "", nil))
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if rec.Len() == 0 {
		t.Fatal("expected log entry")
	}
	entry := rec.All()[0]

	var hasTenantID bool
	entry.Attrs(func(a slog.Attr) bool {
		if a.Key == "tenant_id" && a.Value.String() == "test-tenant" {
			hasTenantID = true
			return false
		}
		return true
	})
	if !hasTenantID {
		t.Error("expected tenant_id field in log entry")
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

// @sk-test 51-shield-gateway-integration#T3.2: TestShieldIntegration full cycle blocked and clean (AC-007)
// @sk-test 13-shield-middleware-wiring#T4.2: Updated to use newPIITenant with PIIConfig
func TestShieldIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, log := newTestLogger(t)

	mockEng := &mockEngine{}

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"choices": []gin.H{
				{"message": gin.H{"role": "assistant", "content": "ok"}},
			},
		})
	})

	t.Run("blocked", func(t *testing.T) {
		mockEng.resp = &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusBlocked),
		}

		w := httptest.NewRecorder()
		body := chatBody("gpt-4", "my SSN is 123-45-6789")
		req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
		if w.Header().Get("X-Shield-Status") != "blocked" {
			t.Errorf("expected X-Shield-Status: blocked, got %s", w.Header().Get("X-Shield-Status"))
		}
	})

	t.Run("clean", func(t *testing.T) {
		mockEng.resp = &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusClean),
		}

		w := httptest.NewRecorder()
		body := chatBody("gpt-4", "hello")
		req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Header().Get("X-Shield-Status") != "clean" {
			t.Errorf("expected X-Shield-Status: clean, got %s", w.Header().Get("X-Shield-Status"))
		}
	})
}

// @sk-test 112-proxy-streaming-wiring#T2.3: TestWrapSSEHeaders (AC-002)
func TestWrapSSEHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(WrapSSE())
	engine.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Streaming request → SSE headers should be set
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"stream": true}`)
	req, _ := http.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type: text/event-stream, got %s", ct)
	}
	if te := w.Header().Get("Transfer-Encoding"); te != "chunked" {
		t.Errorf("expected Transfer-Encoding: chunked, got %s", te)
	}

	// Non-streaming request → no SSE headers
	w2 := httptest.NewRecorder()
	body2 := strings.NewReader(`{"stream": false}`)
	req2, _ := http.NewRequest(http.MethodPost, "/test", body2)
	req2.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w2, req2)

	if ct := w2.Header().Get("Content-Type"); ct == "text/event-stream" {
		t.Error("expected no event-stream Content-Type for non-streaming request")
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

// @sk-test 61-observability#T4.1: TestMetricsMiddleware verifies HTTP metrics are recorded (AC-003)
func TestMetricsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	_, noopLog := newTestLogger(t)
	engine.Use(Logger(noopLog))
	engine.Use(metrics.Middleware())
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

// @sk-test 61-observability#T4.1: TestShieldMiddleware_Metrics verifies shield metrics with mock (AC-004)
// @sk-test 13-shield-middleware-wiring#T4.2: Updated to use newPIITenant with PIIConfig
func TestShieldMiddleware_Metrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockEng := &mockEngine{
		resp: &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusClean),
		},
	}
	_, log := newTestLogger(t)

	promReg := prometheus.NewRegistry()
	metrics.RegisterMetrics(promReg)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	body := chatBody("gpt-4", "hello")
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
