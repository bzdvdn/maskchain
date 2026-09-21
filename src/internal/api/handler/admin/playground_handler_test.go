package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// @sk-test ui-playground: TestPlaygroundHandlerForwards (AC-001)
func TestPlaygroundHandlerForwards(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Shield-Status", "clean")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"}}]}`))
	}))
	defer upstream.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewPlaygroundHandler(upstream.URL)
	router.POST("/api/v1/admin/playground", h.Handle)

	body := `{"api_key":"sk-mc_test","model":"llama3.2","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/playground", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer sk-mc_test" {
		t.Errorf("expected bearer key forwarded, got %q", gotAuth)
	}
	if gotBody["model"] != "llama3.2" || gotBody["stream"] != false {
		t.Errorf("unexpected forwarded body: %v", gotBody)
	}

	var resp struct {
		Status       int    `json:"status"`
		ShieldStatus string `json:"shield_status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != http.StatusOK || resp.ShieldStatus != "clean" {
		t.Errorf("unexpected relay response: %+v", resp)
	}
}

// @sk-test ui-playground: TestPlaygroundHandlerUnreachable (AC-001)
func TestPlaygroundHandlerUnreachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Reserved TEST-NET address that refuses connections quickly.
	h := NewPlaygroundHandler("http://127.0.0.1:1")
	router.POST("/api/v1/admin/playground", h.Handle)

	body := `{"api_key":"k","model":"m","messages":[{"role":"user","content":"x"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/playground", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}
