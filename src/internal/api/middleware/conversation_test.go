package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	conversationapp "github.com/bzdvdn/maskchain/src/internal/app/conversation"
	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type recordingSender struct {
	records []conversationapp.RawRecord
}

func (s *recordingSender) Send(rec conversationapp.RawRecord) {
	s.records = append(s.records, rec)
}

func newTestConversationMiddleware(t *testing.T) (*ConversationMiddleware, *recordingSender) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sender := &recordingSender{}
	log := slog.New(slog.NewTextHandler(nopWriter{}, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	return NewConversationMiddleware(sender, log), sender
}

func testTenantCtx(c *gin.Context) {
	slug, _ := value.NewTenantSlug("tenant-a")
	c.Set(tenantKey, entity.NewTenant(slug, "Tenant A", "", nil))
}

// @sk-test conversation-logging#T2.2: TestConversationMiddlewareCapturesExchange (AC-001, AC-010)
func TestConversationMiddlewareCapturesExchange(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { testTenantCtx(c); c.Next() })
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Header("X-Shield-Status", "clean")
		c.Set(conversationMaskKey, map[string]string{"[MASK_A.0]": "secret@example.com"})
		c.Data(http.StatusOK, "application/json", []byte(`{"choices":[{"message":{"content":"masked answer"}}]}`))
	})

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"choices":[{"message":{"content":"masked answer"}}]}` {
		t.Errorf("response forwarded incorrectly: %s", w.Body.String())
	}

	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if rec.TenantID != "tenant-a" {
		t.Errorf("TenantID = %q, want tenant-a", rec.TenantID)
	}
	if rec.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", rec.Model)
	}
	if rec.Status != conversation.StatusOK {
		t.Errorf("Status = %q, want ok", rec.Status)
	}
	if string(rec.Request) != body {
		t.Errorf("cached request = %q, want original", rec.Request)
	}
	if string(rec.Response) != `{"choices":[{"message":{"content":"masked answer"}}]}` {
		t.Errorf("cached response = %q", rec.Response)
	}
	if len(rec.Masking) != 1 {
		t.Errorf("expected 1 masking entry, got %+v", rec.Masking)
	}
}

// @sk-test conversation-logging#T6.1: TestConversationMiddlewareSkipsUnmaskedClean (user requirement)
func TestConversationMiddlewareSkipsUnmaskedClean(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Header("X-Shield-Status", "clean")
		c.Data(http.StatusOK, "application/json", []byte(`{"choices":[{"message":{"content":"plain answer"}}]}`))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(sender.records) != 0 {
		t.Fatalf("expected 0 records for clean unmasked request, got %d", len(sender.records))
	}
}

// @sk-test conversation-logging#T2.2: TestConversationMiddlewareMaskingFromContext (RQ-007, AC-001)
func TestConversationMiddlewareMaskingFromContext(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { testTenantCtx(c); c.Next() })
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Set(conversationMaskKey, map[string]string{"[MASK_A.0]": "secret@example.com"})
		c.Header("X-Shield-Status", "clean")
		c.Data(http.StatusOK, "application/json", []byte(`{"ok":true}`))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if len(rec.Masking) != 1 || rec.Masking[0].Placeholder != "[MASK_A.0]" || rec.Masking[0].Original != "secret@example.com" {
		t.Errorf("unexpected masking: %+v", rec.Masking)
	}
}

// @sk-test conversation-logging#T2.2: TestConversationMiddlewareBlockedStatus (AC-010)
// @sk-task conversation-logging#T6.1: Blocked requests are logged even without masking (user requirement)
func TestConversationMiddlewareBlockedStatus(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		c.Header("X-Shield-Status", "blocked")
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"shield_status": "blocked"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if rec.Status != conversation.StatusBlocked {
		t.Errorf("Status = %q, want blocked", rec.Status)
	}
}

// @sk-test conversation-logging#T2.2: TestConversationMiddlewareStreamCaptureAndEmptySkip (AC-001)
// @sk-test conversation-logging#T2.5: TestConversationMiddlewareStreamCaptureAndEmptySkip (RQ-009, DEC-007, AC-003)
func TestConversationMiddlewareStreamCaptureAndEmptySkip(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { testTenantCtx(c); c.Next() })
	engine.Use(mw.Handler())
	engine.POST("/stream", func(c *gin.Context) {
		c.Header("X-Shield-Status", "clean")
		c.Set(conversationMaskKey, map[string]string{"[MASK_A.0]": "hi"})
		c.Writer.WriteString("data: {\"chunk\":1}\n\ndata: [DONE]\n\n")
	})
	engine.POST("/empty", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(`{}`))
	})

	t.Run("empty body", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/empty", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		if len(sender.records) != 0 {
			t.Errorf("expected no records for empty body, got %d", len(sender.records))
		}
	})

	t.Run("streaming captured verbatim", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/stream",
			strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		if len(sender.records) != 1 {
			t.Fatalf("expected 1 record for streaming, got %d", len(sender.records))
		}
		rec := sender.records[0]
		if !rec.Streamed {
			t.Error("expected Streamed=true for streaming request")
		}
		if rec.Status != conversation.StatusOK {
			t.Errorf("Status = %q, want ok", rec.Status)
		}
		if string(rec.Response) != "data: {\"chunk\":1}\n\ndata: [DONE]\n\n" {
			t.Errorf("cached stream = %q, want verbatim SSE", rec.Response)
		}
	})
}

// @sk-test conversation-logging#T2.5: TestConversationMiddlewareStreamWithoutDoneIsError (RQ-009, DEC-007, AC-003)
// @sk-task conversation-logging#T6.1: Masked stream without [DONE] is logged as error (user requirement)
func TestConversationMiddlewareStreamWithoutDoneIsError(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/stream", func(c *gin.Context) {
		c.Header("X-Shield-Status", "clean")
		c.Set(conversationMaskKey, map[string]string{"[MASK_A.0]": "hi"})
		c.Data(http.StatusOK, "application/json", []byte(`data: {"chunk":1}`))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/stream",
		strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if !rec.Streamed {
		t.Error("expected Streamed=true")
	}
	if rec.Status != conversation.StatusError {
		t.Errorf("Status = %q, want error for stream without [DONE]", rec.Status)
	}
}

// @sk-test conversation-logging#T2.5: TestConversationMiddlewareStreamBufferCap (RQ-009, DEC-007, AC-003)
func TestConversationMiddlewareStreamBufferCap(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/stream", func(c *gin.Context) {
		c.Header("X-Shield-Status", "clean")
		c.Set(conversationMaskKey, map[string]string{"[MASK_A.0]": "hi"})
		chunk := bytes.Repeat([]byte("x"), 64*1024)
		for i := 0; i < 20; i++ {
			c.Writer.Write(chunk)
		}
		c.Writer.WriteString("data: [DONE]\n\n")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/stream",
		strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	wantLen := 20*64*1024 + len("data: [DONE]\n\n")
	if w.Body.Len() != wantLen {
		t.Errorf("client received %d bytes, want %d (stream must not be stalled)", w.Body.Len(), wantLen)
	}
	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if !rec.Streamed {
		t.Error("expected Streamed=true")
	}
	if len(rec.Response) != 1<<20 {
		t.Errorf("capped response = %d bytes, want 1MB", len(rec.Response))
	}
}

// @sk-test conversation-logging#T2.5: TestConversationMiddlewareStreamBlocked (RQ-009, DEC-007, AC-003)
func TestConversationMiddlewareStreamBlocked(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)

	engine := gin.New()
	engine.Use(mw.Handler())
	engine.POST("/stream", func(c *gin.Context) {
		c.Header("X-Shield-Status", "blocked")
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"shield_status": "blocked"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/stream",
		strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if !rec.Streamed {
		t.Error("expected Streamed=true")
	}
	if rec.Status != conversation.StatusBlocked {
		t.Errorf("Status = %q, want blocked", rec.Status)
	}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// @sk-test conversation-logging#T4.1: TestConversationMiddlewareShieldIntegrationCapture (AC-001)
func TestConversationMiddlewareShieldIntegrationCapture(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)
	_, log := newTestLogger(t)
	mockEng := &mockEngine{
		resp: &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusClean),
		},
	}

	dict := dictionary.NewDictionary("names", []interface{}{"original-name"}, dictionary.MatchModeExact)
	slug, _ := value.NewTenantSlug("tenant-a")
	tenant := entity.NewTenant(slug, "Tenant A", "Authorization", nil,
		entity.WithTenantDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled: true,
			Rules:   []entity.PIARule{{Label: "detect", Type: "regex", Pattern: "SOME", Action: "block"}},
		}),
	)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set(tenantKey, tenant); c.Next() })
	engine.Use(mw.Handler())
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		var chatReq chatRequest
		json.Unmarshal(bodyBytes, &chatReq)
		last := chatReq.Messages[len(chatReq.Messages)-1]
		c.JSON(http.StatusOK, gin.H{
			"choices": []gin.H{
				{"message": gin.H{"role": "assistant", "content": "The employee " + last.Content + " did great"}},
			},
		})
	})

	w := httptest.NewRecorder()
	body := chatBody("gpt-4", "tell me about original-name")
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "original-name") {
		t.Errorf("response must be unmasked to the client: %s", w.Body.String())
	}
	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if string(rec.Request) != body {
		t.Errorf("captured request = %q, want original pre-mask body", rec.Request)
	}
	if rec.Status != conversation.StatusOK {
		t.Errorf("Status = %q, want ok", rec.Status)
	}
	if len(rec.Masking) != 1 {
		t.Fatalf("expected 1 masking entry from shield, got %+v", rec.Masking)
	}
	if rec.Masking[0].Original != "original-name" {
		t.Errorf("masking original = %q, want original-name", rec.Masking[0].Original)
	}
	if rec.MaskID == "" {
		t.Error("expected MaskID to be propagated from shield dict mask")
	}
}

// @sk-test conversation-logging#T4.1: TestConversationMiddlewareShieldIntegrationBlocked (AC-002)
func TestConversationMiddlewareShieldIntegrationBlocked(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)
	_, log := newTestLogger(t)
	mockEng := &mockEngine{
		resp: &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusBlocked),
		},
	}

	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set(tenantKey, newPIITenant("tenant-a")); c.Next() })
	engine.Use(mw.Handler())
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		t.Error("handler must not be called when shield blocks")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "my SSN is 123-45-6789")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if rec.Status != conversation.StatusBlocked {
		t.Errorf("Status = %q, want blocked", rec.Status)
	}
}

// @sk-test conversation-logging#T4.1: TestConversationMiddlewareShieldIntegrationStream (AC-003)
func TestConversationMiddlewareShieldIntegrationStream(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)
	_, log := newTestLogger(t)
	mockEng := &mockEngine{
		resp: &appshield.ScanResponse{
			ScanResult: entity.NewScanResult(value.ScanStatusClean),
		},
	}

	dict := dictionary.NewDictionary("names", []interface{}{"original-name"}, dictionary.MatchModeExact)
	slug, _ := value.NewTenantSlug("tenant-a")
	tenant := entity.NewTenant(slug, "Tenant A", "Authorization", nil,
		entity.WithTenantDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithTenantPIIConfig(entity.PIIConfig{Enabled: false}),
	)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set(tenantKey, tenant); c.Next() })
	engine.Use(mw.Handler())
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		var chatReq chatRequest
		json.Unmarshal(bodyBytes, &chatReq)
		last := chatReq.Messages[len(chatReq.Messages)-1]
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Write([]byte(fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"The person %s did great\"}}]}\n\n", last.Content)))
		c.Writer.Write([]byte("data: [DONE]\n\n"))
	})

	w := httptest.NewRecorder()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"tell me about original-name"}],"stream":true}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "original-name") {
		t.Errorf("stream must be unmasked to the client: %s", w.Body.String())
	}
	if len(sender.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sender.records))
	}
	rec := sender.records[0]
	if !rec.Streamed {
		t.Error("expected Streamed=true")
	}
	if rec.Status != conversation.StatusOK {
		t.Errorf("Status = %q, want ok", rec.Status)
	}
	if string(rec.Response) != "data: {\"choices\":[{\"delta\":{\"content\":\"The person tell me about original-name did great\"}}]}\n\ndata: [DONE]\n\n" {
		t.Errorf("captured stream = %q, want verbatim unmasked SSE", rec.Response)
	}
}

// @sk-test conversation-logging#T4.1: TestConversationMiddlewareShieldIntegrationFailOpen (AC-010)
// @sk-task conversation-logging#T6.1: Unmasked fail-open request is not logged (user requirement)
func TestConversationMiddlewareShieldIntegrationFailOpen(t *testing.T) {
	mw, sender := newTestConversationMiddleware(t)
	_, log := newTestLogger(t)
	mockEng := &mockEngine{err: fmt.Errorf("scan service unavailable")}

	var handlerCalled bool
	slug, _ := value.NewTenantSlug("tenant-a")
	tenant := entity.NewTenant(slug, "Tenant A", "Authorization", nil,
		entity.WithTenantPIIConfig(entity.PIIConfig{
			Enabled:       true,
			DefaultAction: "allow",
			Rules:         []entity.PIARule{{Label: "detect", Type: "regex", Pattern: "SOME", Action: "block"}},
		}),
	)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set(tenantKey, tenant); c.Next() })
	engine.Use(mw.Handler())
	engine.Use(ShieldMiddleware(mockEng, testShieldConfig(), log))
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"choices": []gin.H{{"message": gin.H{"content": "ok"}}}})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(chatBody("gpt-4", "hello")))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called on shield engine error (fail-open)")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(sender.records) != 0 {
		t.Fatalf("expected 0 records for unmasked fail-open, got %d", len(sender.records))
	}
}
