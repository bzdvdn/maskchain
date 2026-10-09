package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

func responsesShieldEngine(t *testing.T, tenant *entity.Tenant, eng Scanner, handler gin.HandlerFunc) (*gin.Engine, *embeddingsCapture) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	capture := &embeddingsCapture{}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(tenantKey, tenant)
		c.Next()
	})
	engine.Use(ResponsesShieldMiddleware(eng, nil, slog.Default()))
	engine.POST("/v1/responses", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		capture.body = b
		capture.called = true
		if handler != nil {
			handler(c)
			return
		}
		c.JSON(http.StatusOK, gin.H{"object": "response"})
	})
	return engine, capture
}

func postResponses(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test responses-api#T2.2: instructions and input text are masked (AC-003)
func TestResponsesShieldMasksRequest(t *testing.T) {
	engine, capture := responsesShieldEngine(t, embeddingsTenant(t), nil, nil)
	w := postResponses(engine, `{"model":"m","instructions":"hello SecretProject","input":"about SecretProject"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := string(capture.body)
	if !strings.Contains(body, "[MASK.0]") {
		t.Errorf("expected placeholders in masked request, got %s", body)
	}
	if strings.Contains(body, "SecretProject") {
		t.Errorf("dictionary term leaked: %s", body)
	}
}

// @sk-test responses-api#T2.2: response text is unmasked (AC-004)
func TestResponsesShieldUnmasksResponse(t *testing.T) {
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"object": "response", "output_text": "hi [MASK.0]"})
	}
	engine, _ := responsesShieldEngine(t, embeddingsTenant(t), nil, handler)

	w := postResponses(engine, `{"model":"m","input":"about SecretProject"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "SecretProject") {
		t.Errorf("response was not unmasked: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "[MASK.0]") {
		t.Errorf("placeholder left in response: %s", w.Body.String())
	}
}

// @sk-test responses-streaming#T1.2: streaming is accepted and the mapping is exposed (AC-002, AC-003)
func TestResponsesShieldAcceptsStreamAndExposesMapping(t *testing.T) {
	var gotMapping map[string]string
	handler := func(c *gin.Context) {
		gotMapping = ResponsesMaskFromContext(c)
		c.Status(http.StatusOK)
	}
	engine, capture := responsesShieldEngine(t, embeddingsTenant(t), nil, handler)

	w := postResponses(engine, `{"model":"m","input":"about SecretProject","stream":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotMapping["[MASK.0]"] != "SecretProject" {
		t.Errorf("mapping not exposed to the handler: %v", gotMapping)
	}
	if !bytes.Contains(capture.body, []byte("[MASK.0]")) || bytes.Contains(capture.body, []byte("SecretProject")) {
		t.Errorf("streaming request was not masked: %s", capture.body)
	}
}

// @sk-test responses-api#T2.2: non-text fields are preserved (AC-005)
func TestResponsesShieldPreservesFields(t *testing.T) {
	engine, capture := responsesShieldEngine(t, embeddingsTenant(t), nil, nil)
	body := `{"model":"m","input":"about SecretProject","store":true,"previous_response_id":"resp_1","tools":[{"type":"web_search"}]}`
	w := postResponses(engine, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(capture.body, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(raw["store"]) != "true" {
		t.Errorf("store changed: %s", raw["store"])
	}
	var prev string
	_ = json.Unmarshal(raw["previous_response_id"], &prev)
	if prev != "resp_1" {
		t.Errorf("previous_response_id changed: %q", prev)
	}
	if !strings.Contains(string(raw["tools"]), "web_search") {
		t.Errorf("tools changed: %s", raw["tools"])
	}
}

// @sk-test responses-api#T2.2: extractor handles instructions + input shapes (AC-003, AC-005)
func TestResponsesFieldsShapes(t *testing.T) {
	body := `{"model":"m","instructions":"hi SecretProject","input":[{"role":"user","content":[{"type":"input_text","text":"a SecretProject"}]},{"type":"input_text","text":"b"}]}`
	texts, apply, err := responsesFields([]byte(body))
	if err != nil {
		t.Fatalf("responsesFields: %v", err)
	}
	want := []string{"hi SecretProject", "a SecretProject", "b"}
	if len(texts) != len(want) {
		t.Fatalf("texts = %v, want %v", texts, want)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Fatalf("texts[%d] = %q, want %q", i, texts[i], want[i])
		}
	}

	out := apply([]string{"I", "A", "B"})
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(out, &raw)
	var inst string
	_ = json.Unmarshal(raw["instructions"], &inst)
	if inst != "I" {
		t.Errorf("instructions = %q", inst)
	}
	var items []map[string]json.RawMessage
	_ = json.Unmarshal(raw["input"], &items)
	var blocks []map[string]json.RawMessage
	_ = json.Unmarshal(items[0]["content"], &blocks)
	var b0 string
	_ = json.Unmarshal(blocks[0]["text"], &b0)
	if b0 != "A" {
		t.Errorf("input[0].content[0].text = %q", b0)
	}
	var b1 string
	_ = json.Unmarshal(items[1]["text"], &b1)
	if b1 != "B" {
		t.Errorf("input[1].text = %q", b1)
	}
}
