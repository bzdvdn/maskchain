package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestAgentHandler(t *testing.T, upstream, card string) *AgentHandler {
	t.Helper()
	h, err := NewAgentHandler(upstream, "k", "Authorization", "Bearer ", card, "5s", nil)
	if err != nil {
		t.Fatalf("NewAgentHandler: %v", err)
	}
	if h == nil {
		t.Fatal("handler is nil")
	}
	return h
}

// @sk-test mcp-a2a-gateway#T2.3: JSON-RPC reply is relayed and unmasked with upstream auth (AC-001, AC-003, AC-009)
func TestAgentHandlerRelaysJSON(t *testing.T) {
	var gotAuth string
	var gotBody []byte
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"text":"hi [MASK.0]"}]}}`))
	}))
	defer up.Close()

	h := newTestAgentHandler(t, up.URL, "")
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call"}`))
	c.Set("agent_mask_mapping", map[string]string{"[MASK.0]": "secret"})

	h.Handle(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer k" {
		t.Errorf("upstream auth = %q", gotAuth)
	}
	if string(gotBody) != `{"jsonrpc":"2.0","id":1,"method":"tools/call"}` {
		t.Errorf("upstream body = %s", gotBody)
	}
	if !strings.Contains(w.Body.String(), "hi secret") || strings.Contains(w.Body.String(), "[MASK.0]") {
		t.Errorf("reply not unmasked: %s", w.Body.String())
	}
}

// @sk-test mcp-a2a-gateway#T2.3: SSE reply is relayed and unmasked (AC-005)
func TestAgentHandlerRelaysSSE(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"result\":{\"content\":[{\"text\":\"[MASK.0]\"}]}}\n\n"))
	}))
	defer up.Close()

	h := newTestAgentHandler(t, up.URL, "")
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/mcp", func(c *gin.Context) {
		c.Set("agent_mask_mapping", map[string]string{"[MASK.0]": "secret"})
		h.Handle(c)
	})
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "secret") || strings.Contains(string(b), "[MASK.0]") {
		t.Errorf("SSE not unmasked: %s", b)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
}

// @sk-test mcp-a2a-gateway#T2.3: A2A agent card is proxied (AC-007)
func TestAgentHandlerCard(t *testing.T) {
	card := `{"name":"agent-x","url":"http://x"}`
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(card))
	}))
	defer up.Close()

	h := newTestAgentHandler(t, up.URL, up.URL+"/.well-known/agent.json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/.well-known/agent.json", nil)

	h.HandleAgentCard(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != card {
		t.Errorf("card = %s", w.Body.String())
	}
}
