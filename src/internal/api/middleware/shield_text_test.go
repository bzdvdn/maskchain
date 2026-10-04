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
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test openai-endpoint-coverage#T2.3: moderations input (string and array) (AC-003)
func TestModerationsFields(t *testing.T) {
	tf, err := textFieldsFor(InputModerations, []byte(`{"model":"m","input":"hello SecretProject","extra":1}`))
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	if len(tf.Texts) != 1 || tf.Texts[0] != "hello SecretProject" {
		t.Fatalf("texts = %v", tf.Texts)
	}

	out := tf.Rewrite([]string{"hello [MASK.0]"})
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var got string
	if err := json.Unmarshal(raw["input"], &got); err != nil {
		t.Fatalf("input: %v", err)
	}
	if got != "hello [MASK.0]" {
		t.Errorf("input = %q, want masked", got)
	}
	if string(raw["extra"]) != "1" {
		t.Errorf("extra field changed: %s", raw["extra"])
	}

	tf2, err := textFieldsFor(InputModerations, []byte(`{"model":"m","input":["a","b"]}`))
	if err != nil {
		t.Fatalf("fields array: %v", err)
	}
	if len(tf2.Texts) != 2 {
		t.Fatalf("texts = %v", tf2.Texts)
	}
	out2 := tf2.Rewrite([]string{"x", "y"})
	var raw2 map[string]json.RawMessage
	_ = json.Unmarshal(out2, &raw2)
	var arr []string
	if err := json.Unmarshal(raw2["input"], &arr); err != nil {
		t.Fatalf("array input: %v (%s)", err, out2)
	}
	if len(arr) != 2 || arr[0] != "x" || arr[1] != "y" {
		t.Errorf("array input = %v", arr)
	}
}

// @sk-test openai-endpoint-coverage#T2.3: rerank query + documents masking (AC-004)
func TestRerankFields(t *testing.T) {
	body := `{"model":"rr","query":"find SecretProject","documents":[{"text":"doc about SecretProject","id":1},"plain SecretProject"],"top_n":2}`
	tf, err := textFieldsFor(InputRerank, []byte(body))
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	want := []string{"find SecretProject", "doc about SecretProject", "plain SecretProject"}
	if len(tf.Texts) != len(want) {
		t.Fatalf("texts = %v, want %v", tf.Texts, want)
	}
	for i := range want {
		if tf.Texts[i] != want[i] {
			t.Fatalf("texts[%d] = %q, want %q", i, tf.Texts[i], want[i])
		}
	}

	out := tf.Rewrite([]string{"find [MASK.0]", "doc about [MASK.1]", "plain [MASK.2]"})
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var query string
	_ = json.Unmarshal(raw["query"], &query)
	if query != "find [MASK.0]" {
		t.Errorf("query = %q", query)
	}
	var docs []json.RawMessage
	_ = json.Unmarshal(raw["documents"], &docs)
	var doc0 map[string]json.RawMessage
	_ = json.Unmarshal(docs[0], &doc0)
	var doc0Text string
	_ = json.Unmarshal(doc0["text"], &doc0Text)
	if doc0Text != "doc about [MASK.1]" {
		t.Errorf("doc0.text = %q", doc0Text)
	}
	if string(doc0["id"]) != "1" {
		t.Errorf("doc0.id changed: %s", doc0["id"])
	}
	var doc1 string
	_ = json.Unmarshal(docs[1], &doc1)
	if doc1 != "plain [MASK.2]" {
		t.Errorf("doc1 = %q", doc1)
	}
	if string(raw["top_n"]) != "2" {
		t.Errorf("top_n changed: %s", raw["top_n"])
	}
}

// @sk-test openai-endpoint-coverage#T2.3: count_tokens system + messages (string and blocks) (AC-005)
func TestCountTokensFields(t *testing.T) {
	body := `{"model":"claude","system":"sys SecretProject","messages":[{"role":"user","content":"hi SecretProject"},{"role":"assistant","content":[{"type":"text","text":"block SecretProject"}]}]}`
	tf, err := textFieldsFor(InputCountTokens, []byte(body))
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	want := []string{"sys SecretProject", "hi SecretProject", "block SecretProject"}
	if len(tf.Texts) != len(want) {
		t.Fatalf("texts = %v, want %v", tf.Texts, want)
	}

	out := tf.Rewrite([]string{"sys [MASK.0]", "hi [MASK.1]", "block [MASK.2]"})
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var system string
	_ = json.Unmarshal(raw["system"], &system)
	if system != "sys [MASK.0]" {
		t.Errorf("system = %q", system)
	}
	var messages []map[string]json.RawMessage
	_ = json.Unmarshal(raw["messages"], &messages)
	var m0 string
	_ = json.Unmarshal(messages[0]["content"], &m0)
	if m0 != "hi [MASK.1]" {
		t.Errorf("messages[0].content = %q", m0)
	}
	var blocks []map[string]json.RawMessage
	_ = json.Unmarshal(messages[1]["content"], &blocks)
	var blockText string
	_ = json.Unmarshal(blocks[0]["text"], &blockText)
	if blockText != "block [MASK.2]" {
		t.Errorf("messages[1].content[0].text = %q", blockText)
	}
}

func textShieldEngine(t *testing.T, kind InputKind, tenant *entity.Tenant, eng Scanner) (*gin.Engine, *embeddingsCapture) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	capture := &embeddingsCapture{}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(tenantKey, tenant)
		c.Next()
	})
	engine.Use(TextInputShieldMiddleware(kind, eng, nil, slog.Default()))
	engine.POST("/v1/x", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		capture.body = b
		capture.called = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return engine, capture
}

func postJSON(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test openai-endpoint-coverage#T2.3: moderations middleware masks before the handler (AC-003)
func TestTextInputShieldMasksModerations(t *testing.T) {
	engine, capture := textShieldEngine(t, InputModerations, embeddingsTenant(t), nil)
	w := postJSON(engine, `{"model":"m","input":"about SecretProject"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !capture.called {
		t.Fatal("handler was not called")
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(capture.body, &raw)
	var input string
	_ = json.Unmarshal(raw["input"], &input)
	if !strings.Contains(input, "[MASK.0]") || strings.Contains(input, "SecretProject") {
		t.Errorf("expected masked input, got %q", input)
	}
}

// @sk-test openai-endpoint-coverage#T2.3: count_tokens middleware masks message text (AC-005)
func TestTextInputShieldMasksCountTokens(t *testing.T) {
	engine, capture := textShieldEngine(t, InputCountTokens, embeddingsTenant(t), nil)
	w := postJSON(engine, `{"model":"claude","messages":[{"role":"user","content":"about SecretProject"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(capture.body, &raw)
	var messages []map[string]json.RawMessage
	_ = json.Unmarshal(raw["messages"], &messages)
	var content string
	_ = json.Unmarshal(messages[0]["content"], &content)
	if !strings.Contains(content, "[MASK.0]") || strings.Contains(content, "SecretProject") {
		t.Errorf("expected masked content, got %q", content)
	}
}

// @sk-test openai-endpoint-coverage#T2.3: blocked rerank aborts before the handler (AC-004)
func TestTextInputShieldBlocksRerank(t *testing.T) {
	eng := &mockEngine{resp: &appshield.ScanResponse{ScanResult: entity.NewScanResult(value.ScanStatusBlocked)}}
	engine, capture := textShieldEngine(t, InputRerank, embeddingsTenant(t), eng)
	w := postJSON(engine, `{"model":"rr","query":"q","documents":[{"text":"d"}]}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if capture.called {
		t.Error("handler must not be called when blocked")
	}
	if w.Header().Get("X-Shield-Status") != "blocked" {
		t.Errorf("expected X-Shield-Status blocked, got %q", w.Header().Get("X-Shield-Status"))
	}
}

// @sk-test openai-endpoint-coverage#T2.3: unsupported endpoint body is rejected before the handler (AC-003)
func TestTextInputShieldRejectsUnsupportedBody(t *testing.T) {
	engine, capture := textShieldEngine(t, InputModerations, embeddingsTenant(t), nil)
	w := postJSON(engine, `{"model":"m","input":42}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if capture.called {
		t.Error("handler must not be called for an unsupported body")
	}
}
