package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task responses-streaming#T2.1: Responses handler (stream + non-stream) (AC-001, AC-003, AC-004, AC-005)
//
// ResponsesHandler proxies POST /v1/responses. Non-streaming responses reuse the
// model-based relay; streaming responses relay the provider's Responses events,
// reconstructing the event name from the payload type and unmasking the text
// with a carry buffer for placeholders split across chunks.
type ResponsesHandler struct {
	selector *routingSvc.RouteSelector
	fallback *routingSvc.FallbackHandler
}

func NewResponsesHandler(selector *routingSvc.RouteSelector, fallback *routingSvc.FallbackHandler) *ResponsesHandler {
	return &ResponsesHandler{selector: selector, fallback: fallback}
}

func (h *ResponsesHandler) HandleResponses(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
		respondWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "model is required")
		return
	}

	tCtx, _ := middleware.TenantFromContext(c)
	tenantID := ""
	if tCtx != nil {
		tenantID = tCtx.Slug().String()
	}
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}

	_, providers, selErr := h.selector.Select(req.Model, tenantID)
	if errors.Is(selErr, routingSvc.ErrNoRoute) {
		respondWithError(c, http.StatusBadRequest, "NO_ROUTE", "no route for model "+req.Model)
		return
	}
	if len(providers) == 0 {
		respondWithError(c, http.StatusServiceUnavailable, "NO_HEALTHY_PROVIDER", "no healthy provider for model "+req.Model)
		return
	}

	upstreamPath := strings.TrimPrefix(c.Request.URL.Path, "/api")
	providerReq := &ports.ProviderRequest{
		Method:      http.MethodPost,
		URL:         upstreamPath,
		Body:        body,
		Path:        c.Request.URL.Path,
		RawQuery:    c.Request.URL.RawQuery,
		ContentType: c.GetHeader("Content-Type"),
		Headers: map[string]string{
			"X-Tenant-ID": tenantID,
		},
	}

	if req.Stream {
		h.stream(c, providerReq, providers, req.Model)
		return
	}

	resp, providerName, fbErr := h.fallback.Call(c.Request.Context(), providers, providerReq)
	if fbErr != nil {
		if errors.Is(fbErr, ports.ErrUnsupportedEndpoint) {
			respondUnsupportedEndpoint(c, providers, upstreamPath)
			return
		}
		respondWithError(c, http.StatusServiceUnavailable, "NO_HEALTHY_PROVIDER", "no healthy provider for model "+req.Model)
		return
	}
	if resp == nil {
		respondWithError(c, http.StatusServiceUnavailable, "NO_HEALTHY_PROVIDER", "no healthy provider for model "+req.Model)
		return
	}

	c.Header("X-Provider", providerName)
	forwardUpstreamHeaders(c, resp.Headers)
	c.Set(middleware.SkipEnvelopeKey, true)
	ct := headerValue(resp.Headers, "content-type")
	if ct == "" {
		ct = "application/json"
	}
	c.Data(resp.StatusCode, ct, resp.Body)
}

func (h *ResponsesHandler) stream(c *gin.Context, providerReq *ports.ProviderRequest, providers []string, model string) {
	ch, providerName, err := h.fallback.Stream(c.Request.Context(), providers, providerReq)
	if err != nil {
		if errors.Is(err, ports.ErrUnsupportedEndpoint) {
			respondUnsupportedEndpoint(c, providers, providerReq.URL)
			return
		}
		respondWithError(c, http.StatusServiceUnavailable, "NO_HEALTHY_PROVIDER", "no healthy provider for model "+model)
		return
	}

	c.Header("X-Provider", providerName)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Set(middleware.SkipEnvelopeKey, true)

	um := newStreamUnmasker(middleware.ResponsesMaskFromContext(c))
	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case chunk, ok := <-ch:
			if !ok {
				if tail := um.flush(); tail != "" {
					_, _ = io.WriteString(w, tail)
				}
				return false
			}
			if chunk.Err != nil {
				_, _ = io.WriteString(w, errorFrame(chunk.Err.Error()))
				return false
			}
			if chunk.Done {
				if tail := um.flush(); tail != "" {
					_, _ = io.WriteString(w, tail)
				}
				return false
			}
			frame, terminal := um.processEvent(chunk.Data)
			if frame != "" {
				_, _ = io.WriteString(w, frame)
			}
			return !terminal
		}
	})
}

// streamUnmasker restores placeholders in a Responses event stream, holding a
// trailing fragment that could be a split placeholder until the next chunk.
type streamUnmasker struct {
	mapping map[string]string
	carry   string
}

func newStreamUnmasker(mapping map[string]string) *streamUnmasker {
	return &streamUnmasker{mapping: mapping}
}

// processEvent unmask an event payload and returns the SSE frame to emit and
// whether the stream is terminal (response.completed / response.failed).
func (u *streamUnmasker) processEvent(data []byte) (string, bool) {
	var ev map[string]json.RawMessage
	if err := json.Unmarshal(data, &ev); err != nil {
		// Non-JSON payload: relay as-is.
		return sseFrame("", data), false
	}
	typ := jsonString(ev["type"])

	for k, raw := range ev {
		if k != "delta" && k != "text" {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) != nil {
			continue
		}
		masked := u.unmaskFragment(s)
		if b, e := json.Marshal(masked); e == nil {
			ev[k] = b
		}
	}

	out, _ := json.Marshal(ev)
	terminal := typ == "response.completed" || typ == "response.failed"

	frame := ""
	if terminal && u.carry != "" {
		frame += u.flush()
	}
	frame += sseFrame(typ, out)
	return frame, terminal
}

// unmaskFragment prepends the carry, replaces known placeholders, and holds a
// trailing fragment that is a proper prefix of a known placeholder.
func (u *streamUnmasker) unmaskFragment(s string) string {
	combined := u.carry + s
	u.carry = ""
	out := combined
	for ph, original := range u.mapping {
		out = strings.ReplaceAll(out, ph, original)
	}
	if hold := longestHeldSuffix(out, u.mapping); hold != "" {
		u.carry = hold
		out = out[:len(out)-len(hold)]
	}
	return out
}

// flush emits any held tail as a final delta so no text is lost.
func (u *streamUnmasker) flush() string {
	if u.carry == "" {
		return ""
	}
	tail := u.carry
	u.carry = ""
	payload, _ := json.Marshal(map[string]string{"type": "response.output_text.delta", "delta": tail})
	return sseFrame("response.output_text.delta", payload)
}

// longestHeldSuffix returns the longest suffix of s that is a proper prefix of
// one of the placeholder keys.
func longestHeldSuffix(s string, mapping map[string]string) string {
	best := ""
	for ph := range mapping {
		maxLen := len(ph) - 1
		if maxLen > len(s) {
			maxLen = len(s)
		}
		for l := maxLen; l > len(best); l-- {
			if strings.HasSuffix(s, ph[:l]) {
				best = s[len(s)-l:]
				break
			}
		}
	}
	return best
}

// @sk-task mcp-a2a-gateway#T1.2: shared recursive unmask for JSON-RPC agent payloads
//
// recursiveUnmask replaces placeholders in every string value of a decoded JSON
// value (maps, slices, strings) and returns the updated value.
func recursiveUnmask(unmask func(string) string, v any) any {
	switch t := v.(type) {
	case string:
		return unmask(t)
	case []any:
		for i := range t {
			t[i] = recursiveUnmask(unmask, t[i])
		}
		return t
	case map[string]any:
		for k := range t {
			t[k] = recursiveUnmask(unmask, t[k])
		}
		return t
	default:
		return v
	}
}

// unmaskJSONRecursive unmasks all string values of a raw JSON document, returning
// the original bytes when the payload is not valid JSON.
func unmaskJSONRecursive(mapping map[string]string, raw []byte) []byte {
	if len(mapping) == 0 {
		return raw
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	replace := func(s string) string {
		for ph, original := range mapping {
			s = strings.ReplaceAll(s, ph, original)
		}
		return s
	}
	out, err := json.Marshal(recursiveUnmask(replace, v))
	if err != nil {
		return raw
	}
	return out
}

// processData unmasks a streamed JSON payload recursively with the carry buffer
// and returns the bytes to emit; non-JSON is returned unchanged.
func (u *streamUnmasker) processData(data []byte) []byte {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	out, err := json.Marshal(recursiveUnmask(u.unmaskFragment, v))
	if err != nil {
		return data
	}
	return out
}

func sseFrame(event string, data []byte) string {
	if event == "" {
		return "data: " + string(data) + "\n\n"
	}
	return "event: " + event + "\ndata: " + string(data) + "\n\n"
}

func errorFrame(message string) string {
	payload, _ := json.Marshal(map[string]any{
		"type":  "error",
		"error": map[string]string{"message": message},
	})
	return "event: error\ndata: " + string(payload) + "\n\n"
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}
