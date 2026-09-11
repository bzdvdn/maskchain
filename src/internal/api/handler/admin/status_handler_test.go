package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/health"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

func setupStatusEngine(h *StatusHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/admin/status", h.HandleStatus)
	return engine
}

type statusResp struct {
	Data struct {
		Version   string `json:"version"`
		UptimeSec int64  `json:"uptime_seconds"`
		KeyAtRest struct {
			Configured bool   `json:"configured"`
			Cipher     string `json:"cipher"`
		} `json:"key_at_rest"`
		Health struct {
			Status string `json:"status"`
		} `json:"health"`
		ConfigDiff struct {
			Watched  bool     `json:"watched"`
			Sections []string `json:"sections"`
		} `json:"config_diff"`
	} `json:"data"`
}

// @sk-test ui-v2-console#T2.2: status reports version, health and key-at-rest (AC-010)
func TestStatusHandler_ReportsVersionHealthAndKeyAtRest(t *testing.T) {
	t.Setenv(config.KeysKeyEnvVar, "")
	h := NewStatusHandler("v-test", &config.Config{Crypto: &config.CryptoConfig{KeysKey: "base64key=="}}, health.NewService(nil))
	engine := setupStatusEngine(h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/status", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp statusResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Version != "v-test" {
		t.Errorf("expected version v-test, got %q", resp.Data.Version)
	}
	if resp.Data.UptimeSec < 0 {
		t.Errorf("expected a non-negative uptime, got %d", resp.Data.UptimeSec)
	}
	if !resp.Data.KeyAtRest.Configured {
		t.Errorf("expected key_at_rest configured from cfg, got false")
	}
	if resp.Data.Health.Status == "" {
		t.Errorf("expected health status populated")
	}
}

// @sk-test ui-v2-console#T2.2: status reports key-at-rest unconfigured (AC-010)
func TestStatusHandler_KeyAtRestNotConfigured(t *testing.T) {
	t.Setenv(config.KeysKeyEnvVar, "")
	h := NewStatusHandler("v-test", &config.Config{}, health.NewService(nil))
	engine := setupStatusEngine(h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/status", nil)
	engine.ServeHTTP(w, req)

	var resp statusResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.KeyAtRest.Configured {
		t.Errorf("expected key_at_rest unconfigured")
	}
}

// @sk-test ui-v2-console#T2.2: status tolerates a degraded health service (AC-010)
func TestStatusHandler_NilHealthService(t *testing.T) {
	t.Setenv(config.KeysKeyEnvVar, "")
	h := NewStatusHandler("v-test", &config.Config{}, nil)
	engine := setupStatusEngine(h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/status", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
