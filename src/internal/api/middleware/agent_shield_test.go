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

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

func agentShieldEngine(t *testing.T, kind AgentKind, tenant *entity.Tenant, allowed, blocked []string, policies map[string]MCPToolPolicy) (*gin.Engine, *embeddingsCapture) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	capture := &embeddingsCapture{}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(tenantKey, tenant)
		c.Next()
	})
	engine.Use(AgentShieldMiddleware(kind, nil, nil, allowed, blocked, policies, slog.Default()))
	engine.POST("/x", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		capture.body = b
		capture.called = true
		c.Status(http.StatusOK)
	})
	return engine, capture
}

func postAgent(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	return w
}

// @sk-test mcp-a2a-gateway#T2.3: MCP tools/call arguments are masked, envelope preserved (AC-002, AC-009)
func TestAgentShieldMasksMCPArguments(t *testing.T) {
	engine, capture := agentShieldEngine(t, AgentMCP, embeddingsTenant(t), nil, nil, nil)
	body := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"search","arguments":{"q":"SecretProject"}}}`
	w := postAgent(engine, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !capture.called {
		t.Fatal("downstream not called")
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(capture.body, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(env["id"]) != "7" {
		t.Errorf("id changed: %s", env["id"])
	}
	if !strings.Contains(string(capture.body), `"method":"tools/call"`) {
		t.Errorf("method changed: %s", capture.body)
	}
	if !strings.Contains(string(capture.body), "[MASK.0]") || strings.Contains(string(capture.body), "SecretProject") {
		t.Errorf("arguments not masked: %s", capture.body)
	}
	if !strings.Contains(string(capture.body), `"name":"search"`) {
		t.Errorf("tool name not preserved: %s", capture.body)
	}
}

// @sk-test mcp-a2a-gateway#T2.3: a blocked MCP tool is rejected before the upstream (AC-004)
func TestAgentShieldBlocksTool(t *testing.T) {
	engine, capture := agentShieldEngine(t, AgentMCP, embeddingsTenant(t), []string{"search"}, nil, nil)
	body := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"rm","arguments":{}}}`
	w := postAgent(engine, body)
	if capture.called {
		t.Fatal("downstream must not run for a blocked tool")
	}
	if !strings.Contains(w.Body.String(), `-32601`) || !strings.Contains(w.Body.String(), `"id":9`) {
		t.Fatalf("expected JSON-RPC error for tool, got %s", w.Body.String())
	}
}

// @sk-test mcp-a2a-gateway#T2.3: A2A message parts are masked (AC-006)
func TestAgentShieldMasksA2AMessage(t *testing.T) {
	engine, capture := agentShieldEngine(t, AgentA2A, embeddingsTenant(t), nil, nil, nil)
	body := `{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","parts":[{"kind":"text","text":"mail SecretProject"}]}}}`
	w := postAgent(engine, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(string(capture.body), "[MASK.0]") || strings.Contains(string(capture.body), "SecretProject") {
		t.Errorf("A2A message not masked: %s", capture.body)
	}
}

// @sk-test mcp-a2a-gateway#T2.3: a call without arguments passes through unchanged (AC-009)
func TestAgentShieldNoArguments(t *testing.T) {
	engine, capture := agentShieldEngine(t, AgentMCP, embeddingsTenant(t), nil, nil, nil)
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	w := postAgent(engine, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if string(capture.body) != body {
		t.Errorf("body changed: %s", capture.body)
	}
}
