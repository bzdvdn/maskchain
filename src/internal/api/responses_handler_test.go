package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

func newResponsesTestHandler(t *testing.T, client ports.ProviderClient) *ResponsesHandler {
	t.Helper()
	reg, err := routingSvc.NewProviderRegistry(&routing.RoutingConfig{
		Providers: []routing.ProviderConfig{{Name: "openai", BaseURL: "http://openai"}},
		Rules: []routing.RuleConfig{{Tenant: routing.GlobalTenant, Routes: []routing.RouteConfig{
			{Model: "m", Providers: []string{"openai"}},
		}}},
	})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	sel := routingSvc.NewRouteSelector(reg)
	fb := routingSvc.NewFallbackHandler(map[string]ports.ProviderClient{"openai": client})
	return NewResponsesHandler(sel, fb)
}

func handleResponses(t *testing.T, h *ResponsesHandler, body string, mapping map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if mapping != nil {
		c.Set("responses_mask_mapping", mapping)
	}
	h.HandleResponses(c)
	return w
}

// @sk-test responses-streaming#T2.2: carry buffer reassembles split placeholders (AC-003, AC-004)
func TestUnmaskStreamerCarry(t *testing.T) {
	t.Run("whole placeholder", func(t *testing.T) {
		u := newStreamUnmasker(map[string]string{"[MASK.0]": "secret"})
		if got := u.unmaskFragment("hi [MASK.0]!"); got != "hi secret!" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("split placeholder", func(t *testing.T) {
		u := newStreamUnmasker(map[string]string{"[MASK.0]": "secret"})
		if got := u.unmaskFragment("hi [MA"); got != "hi " {
			t.Fatalf("first chunk = %q, want %q", got, "hi ")
		}
		if u.carry != "[MA" {
			t.Fatalf("carry = %q, want [MA", u.carry)
		}
		if got := u.unmaskFragment("SK.0] done"); got != "secret done" {
			t.Fatalf("second chunk = %q, want secret done", got)
		}
	})
	t.Run("flush held tail", func(t *testing.T) {
		u := newStreamUnmasker(map[string]string{"[MASK.0]": "secret"})
		if got := u.unmaskFragment("x [MASK."); got != "x " {
			t.Fatalf("got %q", got)
		}
		frame := u.flush()
		if !strings.Contains(frame, "[MASK.") {
			t.Fatalf("flush lost the tail: %q", frame)
		}
		if u.carry != "" {
			t.Fatalf("carry not cleared: %q", u.carry)
		}
	})
}

// @sk-test responses-streaming#T2.2: streamed events are framed and unmasked (AC-001, AC-003, AC-005)
func TestResponsesHandlerStream(t *testing.T) {
	ch := make(chan ports.ProviderChunk, 3)
	ch <- ports.ProviderChunk{Data: []byte(`{"type":"response.output_text.delta","delta":"hi [MASK.0]"}`)}
	ch <- ports.ProviderChunk{Data: []byte(`{"type":"response.output_text.delta","delta":" end"}`)}
	ch <- ports.ProviderChunk{Data: []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2}}}`)}
	close(ch)

	h := newResponsesTestHandler(t, &mockPortClient{streamCh: ch})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/v1/responses", func(c *gin.Context) {
		c.Set("responses_mask_mapping", map[string]string{"[MASK.0]": "secret"})
		h.HandleResponses(c)
	})
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/responses", "application/json", strings.NewReader(`{"model":"m","input":"x","stream":true}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(body, "event: response.output_text.delta") {
		t.Errorf("missing delta event frame: %s", body)
	}
	if !strings.Contains(body, "event: response.completed") {
		t.Errorf("missing terminal event: %s", body)
	}
	if !strings.Contains(body, "hi secret") {
		t.Errorf("delta text not unmasked: %s", body)
	}
	if strings.Contains(body, "[MASK.0]") {
		t.Errorf("placeholder leaked: %s", body)
	}
	if strings.Contains(body, "[DONE]") {
		t.Errorf("responses stream must not emit [DONE]: %s", body)
	}
}

// @sk-test responses-streaming#T2.2: non-streaming responses still relay (AC-001)
func TestResponsesHandlerNonStream(t *testing.T) {
	mock := &mediaMockClient{status: http.StatusOK, body: []byte(`{"object":"response"}`), headers: map[string]string{"Content-Type": "application/json"}}
	h := newResponsesTestHandler(t, mock)

	w := handleResponses(t, h, `{"model":"m","input":"x"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"object":"response"}` {
		t.Errorf("body = %s", w.Body.String())
	}
	if w.Header().Get("X-Provider") != "openai" {
		t.Errorf("X-Provider = %q", w.Header().Get("X-Provider"))
	}
}

// @sk-test responses-streaming#T2.2: unrouted model returns NO_ROUTE (AC-008)
func TestResponsesHandlerNoRoute(t *testing.T) {
	h := newResponsesTestHandler(t, &mediaMockClient{status: http.StatusOK})
	w := handleResponses(t, h, `{"model":"unknown","input":"x"}`, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "NO_ROUTE") {
		t.Fatalf("expected NO_ROUTE 400, got %d: %s", w.Code, w.Body.String())
	}
}

// @sk-test mcp-a2a-gateway#T1.3: recursive unmask restores nested strings (AC-003, AC-006)
func TestUnmaskJSONRecursive(t *testing.T) {
	raw := []byte(`{"result":{"content":[{"type":"text","text":"hi [MASK.0]"}],"meta":[1,true,"x [MASK.0]"]}}`)
	out := unmaskJSONRecursive(map[string]string{"[MASK.0]": "secret"}, raw)
	if !strings.Contains(string(out), "hi secret") || !strings.Contains(string(out), "x secret") {
		t.Fatalf("nested text not unmasked: %s", out)
	}
	if strings.Contains(string(out), "[MASK.0]") {
		t.Fatalf("placeholder leaked: %s", out)
	}
	if unmaskJSONRecursive(nil, raw) == nil || !strings.Contains(string(unmaskJSONRecursive(map[string]string{}, raw)), "[MASK.0]") {
		t.Error("empty mapping must pass through")
	}
}
