package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

func modelAccessTestRequest(key *virtualkey.VirtualKey, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	tcKey := key
	engine.Use(func(c *gin.Context) {
		if tcKey != nil {
			c.Set(virtualKeyContextKey, tcKey)
		}
		c.Next()
	})
	engine.Use(ModelAccess())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBufferString(body))
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test 300-virtual-keys#T2.2: Unrestricted key allows any model (AC-002)
func TestModelAccessNoScopeAllows(t *testing.T) {
	w := modelAccessTestRequest(fakeKey("", "alpha", nil, nil), `{"model":"gpt-4"}`)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 300-virtual-keys#T2.2: Allowed model passes (AC-002)
func TestModelAccessAllowedPasses(t *testing.T) {
	w := modelAccessTestRequest(fakeKey("", "alpha", []string{"gpt-4"}, nil), `{"model":"gpt-4"}`)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 300-virtual-keys#T2.2: Non-allowed model is rejected (AC-002)
func TestModelAccessDisallowed(t *testing.T) {
	w := modelAccessTestRequest(fakeKey("", "alpha", []string{"gpt-4"}, nil), `{"model":"gpt-3.5"}`)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// @sk-test 300-virtual-keys#T2.2: Blocked model is rejected (AC-002)
func TestModelAccessBlocked(t *testing.T) {
	w := modelAccessTestRequest(fakeKey("", "alpha", nil, []string{"gpt-4"}), `{"model":"gpt-4"}`)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// @sk-test 300-virtual-keys#T2.2: No key in context passes through (AC-002)
func TestModelAccessNoKey(t *testing.T) {
	w := modelAccessTestRequest(nil, `{"model":"gpt-4"}`)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
