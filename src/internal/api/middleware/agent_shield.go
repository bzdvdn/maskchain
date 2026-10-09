package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// @sk-task mcp-a2a-gateway#T2.1: mask mapping shared with the agent handler
const agentMaskKey = "agent_mask_mapping"

// AgentKind selects the JSON-RPC shape masked by the agent shield.
type AgentKind string

const (
	AgentMCP AgentKind = "mcp"
	AgentA2A AgentKind = "a2a"
)

// MCPToolPolicy restricts which MCP tools a tenant may call.
type MCPToolPolicy struct {
	Allowed []string
	Blocked []string
}

// AgentMaskFromContext returns the placeholder→original mapping produced by
// AgentShieldMiddleware, or nil.
func AgentMaskFromContext(c *gin.Context) map[string]string {
	if v, ok := c.Get(agentMaskKey); ok {
		if m, ok := v.(map[string]string); ok {
			return m
		}
	}
	return nil
}

// @sk-task mcp-a2a-gateway#T2.1: agent request shield + MCP tool policy (AC-002, AC-003, AC-004, AC-006, AC-009)
//
// AgentShieldMiddleware masks the text of an MCP `tools/call` (params.arguments)
// or an A2A `message/send` (params.message), stores the mapping for the reply,
// and rejects MCP tools the tenant is not allowed to call. The JSON-RPC envelope
// is preserved.
func AgentShieldMiddleware(kind AgentKind, engine Scanner, cfg *config.ShieldConfig, allowed, blocked []string, tenantPolicies map[string]MCPToolPolicy, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			abortWithShieldError(c, http.StatusBadRequest, "failed to read body", "")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		if len(body) == 0 {
			c.Next()
			return
		}
		if len(body) > maxBodySize {
			abortWithShieldError(c, http.StatusRequestEntityTooLarge, "request body too large", "")
			return
		}

		tenant, ok := TenantFromContext(c)
		if !ok {
			abortWithShieldError(c, http.StatusBadRequest, "missing tenant in context", "")
			return
		}

		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(body, &envelope); err != nil {
			abortWithShieldError(c, http.StatusBadRequest, "invalid JSON-RPC body", "")
			return
		}

		method := rawString(envelope["method"])

		// @sk-task mcp-a2a-gateway#T2.1: MCP tool allow/block (AC-004)
		if kind == AgentMCP && method == "tools/call" {
			name := toolCallName(envelope["params"])
			allow, deny := effectivePolicy(tenant.Slug().String(), allowed, blocked, tenantPolicies)
			if !toolAllowed(name, allow, deny) {
				respondJSONRPCError(c, envelope["id"], -32601, "tool not allowed")
				return
			}
		}

		subtreePath := []string{"params", "arguments"}
		if kind == AgentA2A {
			subtreePath = []string{"params", "message"}
		}

		subtree := nestedRaw(envelope, subtreePath...)
		if len(subtree) == 0 {
			c.Next()
			return
		}
		var subtreeVal any
		if json.Unmarshal(subtree, &subtreeVal) != nil {
			c.Next()
			return
		}
		texts := collectTexts(subtreeVal)
		if len(texts) == 0 {
			c.Next()
			return
		}

		tenantSlug := tenant.Slug().String()
		piiCfg := tenant.PIIConfig()
		start := time.Now()

		masked, dictMasked, mapping, resp, scanErr := maskTexts(c.Request.Context(), engine, tenant, texts)
		if scanErr != nil {
			defaultAction := piiCfg.DefaultAction
			if defaultAction == "" {
				defaultAction = "block"
			}
			log.WarnContext(c.Request.Context(), "agent shield scan failed, applying default_action",
				slog.String("error", scanErr.Error()),
				slog.String("tenant_slug", tenantSlug),
				slog.String("default_action", defaultAction),
			)
			if defaultAction == "block" {
				c.Header("X-Shield-Status", "blocked")
				c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{ShieldStatus: "blocked", Error: "shield scan unavailable, blocked by default action"})
				return
			}
		}

		idx := 0
		subtreeVal = applyTexts(subtreeVal, masked, &idx)
		subtreeOut, err := json.Marshal(subtreeVal)
		if err != nil {
			subtreeOut = subtree
		}
		setNested(envelope, subtreePath, subtreeOut)
		out, err := json.Marshal(envelope)
		if err != nil {
			out = body
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(out))
		c.Set(agentMaskKey, mapping)

		duration := time.Since(start)
		scanStatus := string(respStatus(resp))
		metrics.ShieldScanDuration.WithLabelValues(tenantSlug, scanStatus).Observe(float64(duration.Milliseconds()))
		metrics.ShieldProfilesEvaluated.WithLabelValues(tenantSlug).Inc()
		log.InfoContext(c.Request.Context(), "agent shield scan",
			slog.String("kind", string(kind)),
			slog.String("method", method),
			slog.String("shield_status", scanStatus),
			slog.String("tenant_slug", tenantSlug),
			slog.Int("dict_masked", dictMasked),
			slog.Int("pii_masked", piiMaskedCount(resp)),
			slog.Duration("latency", duration),
		)

		switch respStatus(resp) {
		case value.ScanStatusBlocked:
			c.Header("X-Shield-Status", "blocked")
			publishBlockFacts(c, shieldFindings(resp))
			c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{ShieldStatus: "blocked", Error: "request blocked by content shield"})
		case value.ScanStatusError:
			abortWithShieldError(c, http.StatusBadGateway, "shield scan error", tenantSlug)
		default:
			setShieldCleanHeaders(c)
			c.Next()
		}
	}
}

// effectivePolicy merges the global MCP policy with a tenant override.
func effectivePolicy(slug string, allowed, blocked []string, perTenant map[string]MCPToolPolicy) ([]string, []string) {
	if tp, ok := perTenant[slug]; ok {
		if len(tp.Allowed) > 0 {
			allowed = tp.Allowed
		}
		if len(tp.Blocked) > 0 {
			blocked = tp.Blocked
		}
	}
	return allowed, blocked
}

func toolAllowed(name string, allowed, blocked []string) bool {
	for _, b := range blocked {
		if b == name {
			return false
		}
	}
	if len(allowed) > 0 {
		for _, a := range allowed {
			if a == name {
				return true
			}
		}
		return false
	}
	return true
}

// toolCallName returns the tool name of an MCP tools/call params object.
func toolCallName(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var p struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(params, &p)
	return p.Name
}

func respondJSONRPCError(c *gin.Context, id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"error":{"code":%d,"message":%q}}`, id, code, message)
	c.Header("Content-Type", "application/json")
	c.Set(SkipEnvelopeKey, true)
	c.Abort()
	_, _ = c.Writer.Write([]byte(body))
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

// nestedRaw returns the raw JSON at a nested map path, or nil.
func nestedRaw(envelope map[string]json.RawMessage, path ...string) json.RawMessage {
	var cur json.RawMessage
	for i, key := range path {
		var m map[string]json.RawMessage
		if i == 0 {
			m = envelope
		} else if err := json.Unmarshal(cur, &m); err != nil {
			return nil
		}
		v, ok := m[key]
		if !ok {
			return nil
		}
		cur = v
	}
	return cur
}

// setNested writes value at a nested map path, preserving sibling keys.
func setNested(envelope map[string]json.RawMessage, path []string, value json.RawMessage) {
	cur := value
	for i := len(path) - 1; i >= 0; i-- {
		if i == 0 {
			envelope[path[0]] = cur
			return
		}
		parent := map[string]json.RawMessage{}
		if existing := nestedRaw(envelope, path[:i]...); len(existing) > 0 {
			_ = json.Unmarshal(existing, &parent)
		}
		parent[path[i]] = cur
		cur, _ = json.Marshal(parent)
	}
}

// collectTexts returns every string value of a decoded JSON value in a stable
// (sorted-key) order.
func collectTexts(v any) []string {
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			out = append(out, t)
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, k := range sortedKeys(t) {
				walk(t[k])
			}
		}
	}
	walk(v)
	return out
}

// applyTexts writes the masked strings back into a decoded JSON value following
// the same order as collectTexts.
func applyTexts(v any, masked []string, idx *int) any {
	switch t := v.(type) {
	case string:
		m := masked[*idx]
		*idx++
		return m
	case []any:
		for i := range t {
			t[i] = applyTexts(t[i], masked, idx)
		}
		return t
	case map[string]any:
		for _, k := range sortedKeys(t) {
			t[k] = applyTexts(t[k], masked, idx)
		}
		return t
	default:
		return v
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
