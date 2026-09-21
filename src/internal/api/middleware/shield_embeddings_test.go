package middleware

import (
	"encoding/json"
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

type embeddingsCapture struct {
	body   []byte
	called bool
}

func embeddingsTenant(t *testing.T) *entity.Tenant {
	t.Helper()
	slug, err := value.NewTenantSlug("alpha")
	if err != nil {
		t.Fatalf("slug: %v", err)
	}
	dict := dictionary.NewDictionary("projects", []interface{}{"SecretProject"}, dictionary.MatchModeExact)
	return entity.NewTenant(slug, "Alpha", "Authorization",
		entity.WithTenantDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled:       true,
			DefaultAction: "mask",
			Rules:         []entity.PIARule{{Label: "email", Type: "regex", Pattern: "EMAIL", Action: "mask"}},
		}),
	)
}

func embeddingsEngine(t *testing.T, tenant *entity.Tenant, eng Scanner, handler gin.HandlerFunc) (*gin.Engine, *embeddingsCapture) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	capture := &embeddingsCapture{}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(tenantKey, tenant)
		c.Next()
	})
	engine.Use(EmbeddingsShieldMiddleware(eng, nil, slog.Default()))
	engine.POST("/api/v1/embeddings", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		capture.body = b
		capture.called = true
		if handler != nil {
			handler(c)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": []float64{0.1}})
	})
	return engine, capture
}

func postEmbeddings(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embeddings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	return w
}

func inputField(t *testing.T, body []byte) json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode body: %v (%s)", err, body)
	}
	return raw["input"]
}

// @sk-test embeddings-passthrough#T3.1: dictionary terms are masked in the input (AC-002)
func TestEmbeddingsShieldMasksDictionary(t *testing.T) {
	engine, capture := embeddingsEngine(t, embeddingsTenant(t), nil, nil)

	w := postEmbeddings(engine, `{"model":"emb","input":"tell me about SecretProject"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !capture.called {
		t.Fatal("handler was not called")
	}
	var input string
	if err := json.Unmarshal(inputField(t, capture.body), &input); err != nil {
		t.Fatalf("input: %v", err)
	}
	if !strings.Contains(input, "[MASK.0]") {
		t.Errorf("expected placeholder in masked input, got %q", input)
	}
	if strings.Contains(input, "SecretProject") {
		t.Errorf("dictionary term leaked: %q", input)
	}
}

// @sk-test embeddings-passthrough#T3.1: PII replacements are applied to the input (AC-002)
func TestEmbeddingsShieldMasksPII(t *testing.T) {
	eng := &mockEngine{resp: &appshield.ScanResponse{
		ScanResult:   entity.NewScanResult(value.ScanStatusSuspicious),
		Replacements: map[string]string{"[[pii.0]]": "a@b.com"},
	}}
	engine, capture := embeddingsEngine(t, embeddingsTenant(t), eng, nil)

	w := postEmbeddings(engine, `{"model":"emb","input":"contact a@b.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var input string
	if err := json.Unmarshal(inputField(t, capture.body), &input); err != nil {
		t.Fatalf("input: %v", err)
	}
	if !strings.Contains(input, "[[pii.0]]") || strings.Contains(input, "a@b.com") {
		t.Errorf("expected masked PII input, got %q", input)
	}
}

// @sk-test embeddings-passthrough#T3.1: block rules abort before the provider (AC-003)
func TestEmbeddingsShieldBlocks(t *testing.T) {
	eng := &mockEngine{resp: &appshield.ScanResponse{ScanResult: entity.NewScanResult(value.ScanStatusBlocked)}}
	engine, capture := embeddingsEngine(t, embeddingsTenant(t), eng, nil)

	w := postEmbeddings(engine, `{"model":"emb","input":"my SSN is 123-45-6789"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if capture.called {
		t.Error("handler must not be called when the input is blocked")
	}
	if w.Header().Get("X-Shield-Status") != "blocked" {
		t.Errorf("expected X-Shield-Status blocked, got %q", w.Header().Get("X-Shield-Status"))
	}
}

// @sk-test embeddings-passthrough#T3.1: token arrays and empty input are rejected (AC-004)
func TestEmbeddingsShieldRejectsUnsupportedInput(t *testing.T) {
	engine, capture := embeddingsEngine(t, embeddingsTenant(t), nil, nil)

	for name, body := range map[string]string{
		"token array":  `{"model":"emb","input":[[1,2,3]]}`,
		"empty string": `{"model":"emb","input":""}`,
		"empty array":  `{"model":"emb","input":[]}`,
		"number":       `{"model":"emb","input":42}`,
	} {
		t.Run(name, func(t *testing.T) {
			capture.called = false
			w := postEmbeddings(engine, body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			if capture.called {
				t.Error("handler must not be called for unsupported input")
			}
		})
	}
}

// @sk-test embeddings-passthrough#T3.1: array input stays an array after masking (AC-002)
func TestEmbeddingsShieldArrayInput(t *testing.T) {
	engine, capture := embeddingsEngine(t, embeddingsTenant(t), nil, nil)

	w := postEmbeddings(engine, `{"model":"emb","input":["hello","about SecretProject"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var arr []string
	if err := json.Unmarshal(inputField(t, capture.body), &arr); err != nil {
		t.Fatalf("expected array input, got %s", capture.body)
	}
	if len(arr) != 2 || arr[0] != "hello" || !strings.Contains(arr[1], "[MASK.0]") {
		t.Errorf("unexpected masked array: %v", arr)
	}
}

// @sk-test embeddings-passthrough#T3.1: the response is passed through, never unmasked (AC-002)
func TestEmbeddingsShieldDoesNotUnmaskResponse(t *testing.T) {
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"echo": "[MASK.0]", "data": []float64{0.2}})
	}
	engine, _ := embeddingsEngine(t, embeddingsTenant(t), nil, handler)

	w := postEmbeddings(engine, `{"model":"emb","input":"about SecretProject"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "[MASK.0]") {
		t.Errorf("response must be passed through unchanged, got %s", w.Body.String())
	}
}
