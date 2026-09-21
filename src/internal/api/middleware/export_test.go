package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type recordingExporter struct {
	mu      sync.Mutex
	records []domainlogexport.Record
}

func (r *recordingExporter) Enqueue(rec domainlogexport.Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
}

func (r *recordingExporter) all() []domainlogexport.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domainlogexport.Record, len(r.records))
	copy(out, r.records)
	return out
}

func exportTestEngine(t *testing.T, mode value.RetentionMode, rec ExportRecorder, respBody string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	slug, err := value.NewTenantSlug("alpha")
	if err != nil {
		t.Fatalf("slug: %v", err)
	}
	tenant := entity.NewTenant(slug, "Alpha", "Authorization", entity.WithTenantRetentionMode(mode))
	rates := analytics.NewCostRateRegistry([]*analytics.CostRate{
		{Model: "m", InputPricePer1K: 1, OutputPricePer1K: 2, Currency: "USD"},
	})

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(tenantKey, tenant)
		c.Set(conversationMaskIDKey, "mask-123")
		c.Next()
	})
	engine.Use(ExportMiddleware(rec, rates, slog.Default()))
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(respBody))
	})
	return engine
}

func postExport(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test log-export#T4.4: full retention exports masked content + metadata (AC-002)
func TestExportMiddlewareCapturesMaskedContent(t *testing.T) {
	rec := &recordingExporter{}
	engine := exportTestEngine(t, value.RetentionModeFull, rec,
		`{"usage":{"prompt_tokens":1000,"completion_tokens":500},"choices":[]}`)

	body := `{"model":"m","messages":[{"role":"user","content":"email [MASK.0]"}]}`
	w := postExport(engine, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	records := rec.all()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Tenant != "alpha" || r.Model != "m" || r.MaskID != "mask-123" {
		t.Errorf("unexpected metadata: %+v", r)
	}
	if r.Tokens != 1500 {
		t.Errorf("tokens = %d, want 1500", r.Tokens)
	}
	// 1000/1k*1 + 500/1k*2 = 1 + 1 = 2
	if r.Cost != 2 {
		t.Errorf("cost = %f, want 2", r.Cost)
	}
	if r.Content == nil || r.Content.MaskedRequest != body {
		t.Errorf("expected masked request captured, got %+v", r.Content)
	}
	if r.Content == nil || !strings.Contains(r.Content.MaskedResponse, "prompt_tokens") {
		t.Errorf("expected masked response captured, got %+v", r.Content)
	}
	// The client response is unchanged.
	if !strings.Contains(w.Body.String(), "prompt_tokens") {
		t.Errorf("client response was altered: %s", w.Body.String())
	}
}

// @sk-test log-export#T4.4: meta retention exports metadata only (AC-004)
func TestExportMiddlewareMetaOnly(t *testing.T) {
	rec := &recordingExporter{}
	engine := exportTestEngine(t, value.RetentionModeMeta, rec, `{"usage":{"prompt_tokens":10,"completion_tokens":0}}`)

	postExport(engine, `{"model":"m","messages":[]}`)

	records := rec.all()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Content != nil {
		t.Errorf("meta retention must not export content, got %+v", records[0].Content)
	}
}

// @sk-test log-export#T4.4: none retention exports nothing (AC-004)
func TestExportMiddlewareNoneRetention(t *testing.T) {
	rec := &recordingExporter{}
	engine := exportTestEngine(t, value.RetentionModeNone, rec, `{"usage":{"prompt_tokens":10}}`)

	postExport(engine, `{"model":"m","messages":[]}`)

	if got := len(rec.all()); got != 0 {
		t.Errorf("none retention must export nothing, got %d records", got)
	}
}
