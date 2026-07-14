package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
)

func performWithMiddleware(path string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(ResponseEnvelope())
	r.GET(path, handler)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope wraps success JSON (AC-003)
func TestResponseEnvelopeWrapsSuccess(t *testing.T) {
	w := performWithMiddleware("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"key": "value"})
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var env struct {
		Data  map[string]string `json:"data"`
		Error any               `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if env.Data["key"] != "value" {
		t.Errorf("expected value, got %s", env.Data["key"])
	}
	if env.Error != nil {
		t.Errorf("expected nil error, got %v", env.Error)
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope wraps error JSON (AC-004)
func TestResponseEnvelopeWrapsError(t *testing.T) {
	w := performWithMiddleware("/test", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request", "code": "BAD_REQUEST"})
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var env struct {
		Data  any `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if env.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %s", env.Error.Code)
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope skips non-JSON (AC-010)
func TestResponseEnvelopeSkipsNonJSON(t *testing.T) {
	w := performWithMiddleware("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "plain text")
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "plain text" {
		t.Errorf("expected 'plain text', got %q", w.Body.String())
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope skips health path (AC-003)
func TestResponseEnvelopeSkipsHealth(t *testing.T) {
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(ResponseEnvelope())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected ok, got %s", resp["status"])
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope skips metrics path (AC-003)
func TestResponseEnvelopeSkipsMetrics(t *testing.T) {
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(ResponseEnvelope())
	r.GET("/metrics", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope skips debug paths (AC-003)
func TestResponseEnvelopeSkipsDebug(t *testing.T) {
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(ResponseEnvelope())
	r.GET("/debug/pprof/heap", func(c *gin.Context) {
		c.String(http.StatusOK, "heap profile")
	})
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "heap profile" {
		t.Errorf("expected raw body, got %q", w.Body.String())
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope respects skipEnvelope key (AC-010)
func TestResponseEnvelopeSkipKey(t *testing.T) {
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(ResponseEnvelope())
	r.GET("/test", func(c *gin.Context) {
		c.Set(SkipEnvelopeKey, true)
		c.JSON(http.StatusOK, gin.H{"raw": "data"})
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if resp["raw"] != "data" {
		t.Errorf("expected raw data, got %s", resp["raw"])
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope respects EnvelopedKey (AC-004)
func TestResponseEnvelopeAlreadyEnveloped(t *testing.T) {
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(ResponseEnvelope())
	r.GET("/test", func(c *gin.Context) {
		c.Set(EnvelopedKey, true)
		c.JSON(http.StatusOK, dto.NewSuccessResponse("already enveloped"))
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	var env struct {
		Data  string `json:"data"`
		Error any    `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if env.Data != "already enveloped" {
		t.Errorf("expected 'already enveloped', got %q", env.Data)
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope passes 204 No Content (AC-010)
func TestResponseEnvelopeNoContent(t *testing.T) {
	w := performWithMiddleware("/test", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", w.Body.String())
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope adds pagination from context (AC-005)
func TestResponseEnvelopePagination(t *testing.T) {
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(ResponseEnvelope())
	r.GET("/test", func(c *gin.Context) {
		c.Set("pagination", dto.Pagination{Page: 1, PerPage: 10, Total: 42})
		c.JSON(http.StatusOK, []string{"a", "b"})
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	var env struct {
		Data       []string `json:"data"`
		Pagination struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"pagination"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if len(env.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(env.Data))
	}
	if env.Pagination.Total != 42 {
		t.Errorf("expected total 42, got %d", env.Pagination.Total)
	}
	if env.Error != nil {
		t.Errorf("expected nil error, got %v", env.Error)
	}
}

// @sk-test 118-api-consistency#T4.1: ResponseEnvelope handles 500 error (AC-004)
func TestResponseEnvelopeInternalError(t *testing.T) {
	w := performWithMiddleware("/test", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "server error"})
	})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	var env struct {
		Data  any `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if env.Error.Code == "" {
		t.Errorf("expected non-empty error code, got empty")
	}
}
