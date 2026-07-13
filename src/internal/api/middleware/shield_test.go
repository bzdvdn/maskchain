package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldBlocked returns 403 for critical content (AC-001)
func TestShieldBlocked(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusBlocked, nil),
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newTestTenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, &config.ShieldConfig{}, log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "my SSN is 123-45-6789")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
	if w.Header().Get("X-Shield-Status") != "blocked" {
		t.Errorf("expected X-Shield-Status: blocked, got %s", w.Header().Get("X-Shield-Status"))
	}
	var resp shieldResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ShieldStatus != "blocked" {
		t.Errorf("expected shield_status blocked, got %s", resp.ShieldStatus)
	}
	if resp.IncidentID == "" {
		t.Error("expected non-empty incident_id")
	}
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldClean passes request to handler (AC-002)
func TestShieldClean(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusClean, nil),
	}

	var handlerCalled bool
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newTestTenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, &config.ShieldConfig{}, log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("X-Shield-Status") != "clean" {
		t.Errorf("expected X-Shield-Status: clean, got %s", w.Header().Get("X-Shield-Status"))
	}
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldEngineError returns 502 (AC-005)
func TestShieldEngineError(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.err = nil
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusError, nil),
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newTestTenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, &config.ShieldConfig{}, log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
	if w.Header().Get("X-Shield-Status") != "error" {
		t.Errorf("expected X-Shield-Status: error, got %s", w.Header().Get("X-Shield-Status"))
	}
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldHeaders present in response (AC-006)
func TestShieldHeaders(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusBlocked, nil),
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newTestTenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, &config.ShieldConfig{}, log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "test")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Header().Get("X-Shield-Status") == "" {
		t.Error("expected X-Shield-Status header")
	}
	if w.Header().Get("X-Shield-Incident-ID") == "" {
		t.Error("expected X-Shield-Incident-ID header")
	}
	if _, err := uuid.Parse(w.Header().Get("X-Shield-Incident-ID")); err != nil {
		t.Errorf("expected valid UUID in X-Shield-Incident-ID, got %q", w.Header().Get("X-Shield-Incident-ID"))
	}
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldEmptyMessages passes through (edge case)
func TestShieldEmptyMessages(t *testing.T) {
	engine, mockEng, log := setupTest(t)

	var handlerCalled bool
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newTestTenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, &config.ShieldConfig{}, log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	body, _ := json.Marshal(map[string]interface{}{
		"model":    "gpt-4",
		"messages": []map[string]string{},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called for empty messages")
	}
	if w.Header().Get("X-Shield-Status") != "clean" {
		t.Errorf("expected X-Shield-Status: clean, got %s", w.Header().Get("X-Shield-Status"))
	}
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldNonJSONContentType returns 415 (edge case)
func TestShieldNonJSONContentType(t *testing.T) {
	engine, mockEng, log := setupTest(t)

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newTestTenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, &config.ShieldConfig{}, log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "text/plain")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415, got %d", w.Code)
	}
}

// @sk-test tenant-profile-sync#T3.1: TestShieldMissingTenant returns 400 (AC-006, AC-007)
func TestShieldMissingTenant(t *testing.T) {
	engine, mockEng, log := setupTest(t)

	engine.Use(ShieldMiddleware(mockEng, &config.ShieldConfig{}, log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if w.Header().Get("X-Shield-Status") != "error" {
		t.Errorf("expected X-Shield-Status: error, got %s", w.Header().Get("X-Shield-Status"))
	}
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldLogging checks log fields (AC-008)
func TestShieldLogging(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	log := zap.New(core)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mockEng := &mockEngine{
		resp: &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusClean, nil),
		},
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newTestTenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, &config.ShieldConfig{}, log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if recorded.Len() == 0 {
		t.Fatal("expected log entry")
	}

	var hasStatus, hasSlug, hasModel, hasLatency, hasIncidentID bool
	for _, entry := range recorded.All() {
		for _, f := range entry.Context {
			switch f.Key {
			case "shield_status":
				hasStatus = true
			case "tenant_slug":
				hasSlug = true
			case "model":
				hasModel = true
			case "latency":
				hasLatency = true
			case "incident_id":
				hasIncidentID = true
			}
		}
	}

	if !hasStatus {
		t.Error("expected shield_status in log")
	}
	if !hasSlug {
		t.Error("expected tenant_slug in log")
	}
	if !hasModel {
		t.Error("expected model in log")
	}
	if !hasLatency {
		t.Error("expected latency in log")
	}
	if !hasIncidentID {
		t.Error("expected incident_id in log")
	}
}
