package middleware

// @sk-task cleanup-profile-repository#T3.6: Remove NewProfileSlug/fix NewDictionary in dict-unmask tests (AC-012)
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

func newPIITenant(slug string, opts ...entity.TenantOption) *entity.Tenant {
	s, _ := value.NewTenantSlug(slug)
	return entity.NewTenant(s, "test-"+slug, "Authorization", nil, append([]entity.TenantOption{
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules: []entity.PIARule{
				{Label: "email", Type: "regex", Pattern: "EMAIL", Action: "block"},
			},
		}),
	}, opts...)...)
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldBlocked returns 403 for critical content (AC-001)
func TestShieldBlocked(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusBlocked),
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
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
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldClean passes request to handler (AC-002)
func TestShieldClean(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusClean),
	}

	var handlerCalled bool
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
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
		ScanResult: entity.NewScanResult(value.ScanStatusError),
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
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
		ScanResult: entity.NewScanResult(value.ScanStatusBlocked),
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
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
}

// @sk-test 51-shield-gateway-integration#T2.2: TestShieldEmptyMessages passes through (edge case)
func TestShieldEmptyMessages(t *testing.T) {
	engine, mockEng, log := setupTest(t)

	var handlerCalled bool
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
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
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
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

	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
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
	rec, log := newTestLogger(t)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mockEng := &mockEngine{
		resp: &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusClean),
		},
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
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

	if rec.Len() == 0 {
		t.Fatal("expected log entry")
	}

	var hasStatus, hasSlug, hasPIIEnabled, hasRulesCount, hasModel, hasLatency, hasUnmasked bool
	for _, entry := range rec.All() {
		entry.Attrs(func(a slog.Attr) bool {
			switch a.Key {
			case "shield_status":
				hasStatus = true
			case "tenant_slug":
				hasSlug = true
			case "pii_enabled":
				hasPIIEnabled = true
			case "rules_count":
				hasRulesCount = true
			case "model":
				hasModel = true
			case "latency":
				hasLatency = true
			case "unmasked":
				hasUnmasked = true
			}
			return true
		})
	}

	if !hasStatus {
		t.Error("expected shield_status in log")
	}
	if !hasSlug {
		t.Error("expected tenant_slug in log")
	}
	if !hasPIIEnabled {
		t.Error("expected pii_enabled in log")
	}
	if !hasRulesCount {
		t.Error("expected rules_count in log")
	}
	if !hasModel {
		t.Error("expected model in log")
	}
	if !hasLatency {
		t.Error("expected latency in log")
	}
	if !hasUnmasked {
		t.Error("expected unmasked in log")
	}
}

// @sk-test 13-shield-middleware-wiring#T3.1: TestPIIConfig_BlocksEmail — AC-001 integration
func TestPIIConfig_BlocksEmail(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusBlocked),
	}

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		t.Error("handler should not be called when PII is blocked")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "my email is test@example.com")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
	if w.Header().Get("X-Shield-Status") != "blocked" {
		t.Errorf("expected X-Shield-Status: blocked, got %s", w.Header().Get("X-Shield-Status"))
	}
}

// @sk-test 13-shield-middleware-wiring#T3.1: TestPIIConfig_Disabled — AC-003 integration
func TestPIIConfig_Disabled(t *testing.T) {
	engine, mockEng, log := setupTest(t)

	var handlerCalled bool
	slug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(slug, "test-tenant", "Authorization", nil, entity.WithTenantPIIConfig(entity.PIIConfig{Enabled: false}))
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", tenant)
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called when PIIConfig is disabled")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 13-shield-middleware-wiring#T3.1: TestPIIConfig_EmptyRules — AC-007 integration
func TestPIIConfig_EmptyRules(t *testing.T) {
	engine, mockEng, log := setupTest(t)

	var handlerCalled bool
	slug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(slug, "test-tenant", "Authorization", nil, entity.WithTenantPIIConfig(entity.PIIConfig{
		Enabled: true,
		Rules:   []entity.PIARule{},
	}))
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", tenant)
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called when rules are empty")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 13-shield-middleware-wiring#T3.1: Graceful degradation on engine.Scan error — applies default_action (AC-004)
func TestShieldGracefulDegradation(t *testing.T) {
	t.Run("engine_error_default_block", func(t *testing.T) {
		engine, mockEng, log := setupTest(t)
		mockEng.err = fmt.Errorf("scan service unavailable")

		slug, _ := value.NewTenantSlug("test-tenant")
		tenant := entity.NewTenant(slug, "test-tenant", "Authorization", nil, entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled:       true,
			DefaultAction: "block",
			Rules:         []entity.PIARule{{Label: "email", Type: "regex", Pattern: "EMAIL", Action: "block"}},
		}))
		engine.Use(func(c *gin.Context) {
			c.Set("tenant", tenant)
			c.Next()
		})
		engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
		engine.POST("/v1/chat/completions", func(c *gin.Context) {
			t.Error("handler should not be called when default_action=block")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
		if w.Header().Get("X-Shield-Status") != "blocked" {
			t.Errorf("expected X-Shield-Status: blocked, got %s", w.Header().Get("X-Shield-Status"))
		}
	})

	t.Run("engine_error_default_allow", func(t *testing.T) {
		engine, mockEng, log := setupTest(t)
		mockEng.err = fmt.Errorf("scan service unavailable")
		mockEng.resp = nil

		var handlerCalled bool
		slug, _ := value.NewTenantSlug("test-tenant")
		tenant := entity.NewTenant(slug, "test-tenant", "Authorization", nil, entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled:       true,
			DefaultAction: "allow",
			Rules:         []entity.PIARule{{Label: "email", Type: "regex", Pattern: "EMAIL", Action: "block"}},
		}))
		engine.Use(func(c *gin.Context) {
			c.Set("tenant", tenant)
			c.Next()
		})
		engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
		engine.POST("/v1/chat/completions", func(c *gin.Context) {
			handlerCalled = true
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		if !handlerCalled {
			t.Error("expected handler to be called when default_action=allow")
		}
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

// @sk-test 13-shield-middleware-wiring#T3.2: TestDictUnmask — AC-005 integration
func TestDictUnmask(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusClean),
	}

	dict := dictionary.NewDictionary("names", []interface{}{"original-name"}, dictionary.MatchModeExact)
	tenantSlug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(tenantSlug, "test-tenant", "Authorization", nil,
		entity.WithTenantDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules:   []entity.PIARule{{Label: "detect", Type: "regex", Pattern: "SOME", Action: "block"}},
		}),
	)

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", tenant)
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		buf := new(strings.Builder)
		io.Copy(buf, c.Request.Body)
		var chatReq chatRequest
		json.Unmarshal([]byte(buf.String()), &chatReq)
		last := chatReq.Messages[len(chatReq.Messages)-1]
		echoContent := last.Content
		c.JSON(http.StatusOK, map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"role":    "assistant",
					"content": "The employee " + echoContent + " did a great job!",
				}},
			},
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "tell me about original-name")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "original-name") {
		t.Errorf("expected response to contain unmasked 'original-name', got: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "{{dict.") {
		t.Errorf("expected no dict placeholders in response, got: %s", w.Body.String())
	}
}

// @sk-test 13-shield-middleware-wiring#T3.2: TestStreamingDictUnmask — AC-006 integration
func TestStreamingDictUnmask(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusClean),
	}

	dict := dictionary.NewDictionary("names", []interface{}{"original-name"}, dictionary.MatchModeExact)
	tenantSlug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(tenantSlug, "test-tenant", "Authorization", nil,
		entity.WithTenantDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules:   []entity.PIARule{{Label: "detect", Type: "regex", Pattern: "SOME", Action: "block"}},
		}),
	)

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", tenant)
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		var chatReq chatRequest
		json.Unmarshal(bodyBytes, &chatReq)
		last := chatReq.Messages[len(chatReq.Messages)-1]
		content := last.Content

		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Write([]byte(fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"The person %s did great\"}}]}\n\n", content)))
		c.Writer.Write([]byte("data: [DONE]\n\n"))
	})

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"tell me about original-name"}],"stream":true}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "original-name") {
		t.Errorf("expected response to contain unmasked 'original-name', got: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "{{dict.") {
		t.Errorf("expected no dict placeholders in streaming response, got: %s", w.Body.String())
	}
}

// @sk-test 13-shield-middleware-wiring#T4.1: Tenant without PIIConfig → PII disabled (edge case)
func TestShieldEdge_NoPIIConfig(t *testing.T) {
	engine, mockEng, log := setupTest(t)

	var handlerCalled bool
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newTestTenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called when tenant has no PIIConfig")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 13-shield-middleware-wiring#T4.1: LLM response without placeholders → unmask no-op (edge case)
func TestShieldEdge_UnmaskNoopNoPlaceholders(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusClean),
	}

	dict := dictionary.NewDictionary("names", []interface{}{"original-name"}, dictionary.MatchModeExact)
	tenantSlug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(tenantSlug, "test-tenant", "Authorization", nil,
		entity.WithTenantDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules:   []entity.PIARule{{Label: "detect", Type: "regex", Pattern: "SOME", Action: "block"}},
		}),
	)

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", tenant)
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"role":    "assistant",
					"content": "The employee did a great job!",
				}},
			},
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "tell me about original-name")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "did a great job") {
		t.Errorf("expected unchanged response, got: %s", w.Body.String())
	}
}

// @sk-test 13-shield-middleware-wiring#T4.1: Placeholder with invalid index not in mapping → not replaced (edge case)
func TestShieldEdge_InvalidPlaceholderNotReplaced(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = &appshield.ScanResponse{
		ScanResult: entity.NewScanResult(value.ScanStatusClean),
	}

	dict := dictionary.NewDictionary("names", []interface{}{"original-name"}, dictionary.MatchModeExact)
	tenantSlug, _ := value.NewTenantSlug("test-tenant")
	tenant := entity.NewTenant(tenantSlug, "test-tenant", "Authorization", nil,
		entity.WithTenantDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules:   []entity.PIARule{{Label: "detect", Type: "regex", Pattern: "SOME", Action: "block"}},
		}),
	)

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", tenant)
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		var chatReq chatRequest
		json.Unmarshal(bodyBytes, &chatReq)
		last := chatReq.Messages[len(chatReq.Messages)-1]
		echoContent := last.Content
		c.JSON(http.StatusOK, map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"role":    "assistant",
					"content": "The employee " + echoContent + " did great, ref: {{dict.unknown.99}}",
				}},
			},
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "tell me about original-name")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "original-name") {
		t.Errorf("expected valid placeholder to be unmasked, got: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "{{dict.unknown.99}}") {
		t.Errorf("expected unknown placeholder to remain unchanged, got: %s", w.Body.String())
	}
}

// @sk-test 117-critical-test-coverage#T3.1: TestShieldNilScanResult — no panic when Scan returns (nil,nil) (AC-003)
func TestShieldNilScanResult(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = nil
	mockEng.err = nil

	var handlerCalled bool
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called when Scan returns (nil, nil)")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 117-critical-test-coverage#T3.1: TestShieldContextCancel — no panic on cancelled context (AC-003)
func TestShieldContextCancel(t *testing.T) {
	engine, mockEng, log := setupTest(t)
	mockEng.resp = nil
	mockEng.err = nil

	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))

	var handlerCalled bool
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called even on cancelled context")
	}
}

// @sk-test 117-critical-test-coverage#T3.1: TestShieldNilEngine — nil engine skips scan (disabled shield) (AC-003)
func TestShieldNilEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	_, log := newTestLogger(t)

	var handlerCalled bool
	engine.Use(func(c *gin.Context) {
		c.Set("tenant", newPIITenant("test-tenant"))
		c.Next()
	})
	engine.Use(ShieldMiddleware(nil, testShieldConfig(), log))
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called when engine is nil")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
