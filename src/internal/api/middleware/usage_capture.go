package middleware

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

// @sk-task usage-accounting-integrity#T1.1: Shared JSON/SSE usage capture (AC-001, AC-004)
//
// usageInfo is provider-reported token usage. Providers disagree on field names,
// so both OpenAI (prompt_tokens/completion_tokens) and Anthropic
// (input_tokens/output_tokens) shapes are accepted.
type usageInfo struct {
	PromptTokens     int64
	CompletionTokens int64
}

func (u usageInfo) empty() bool {
	return u.PromptTokens == 0 && u.CompletionTokens == 0
}

const (
	// maxCapturedBody bounds how much of a non-streaming response body is
	// retained for usage extraction. Bodies larger than this are not counted.
	maxCapturedBody = 4 << 20 // 4 MiB
	// maxSSELine bounds a single SSE line so a malformed stream cannot grow
	// memory without limit.
	maxSSELine = 64 << 10 // 64 KiB
)

// usageCapture tees a response so provider usage can be read after the
// response completes, without altering what the client receives. It handles
// both a plain JSON body and an SSE stream.
type usageCapture struct {
	gin.ResponseWriter
	streaming bool
	status    int
	jsonBuf   bytes.Buffer
	truncated bool
	sse       *sseUsageParser
}

// wrapUsageCapture installs a capture writer on the gin context and returns it.
func wrapUsageCapture(c *gin.Context, streaming bool) *usageCapture {
	w := &usageCapture{ResponseWriter: c.Writer, status: 200, streaming: streaming}
	if streaming {
		w.sse = newSSEUsageParser()
	}
	c.Writer = w
	return w
}

func (w *usageCapture) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *usageCapture) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *usageCapture) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *usageCapture) capture(b []byte) {
	if w.streaming {
		if w.sse != nil {
			w.sse.feed(b)
		}
		return
	}
	if w.truncated {
		return
	}
	if w.jsonBuf.Len()+len(b) > maxCapturedBody {
		w.truncated = true
		w.jsonBuf.Reset()
		return
	}
	w.jsonBuf.Write(b)
}

// Usage returns the provider-reported usage, if any was observed.
func (w *usageCapture) Usage() (usageInfo, bool) {
	if w.streaming {
		if w.sse == nil {
			return usageInfo{}, false
		}
		return w.sse.usage()
	}
	if w.truncated {
		return usageInfo{}, false
	}
	return extractJSONUsage(w.jsonBuf.Bytes())
}

// extractJSONUsage reads a usage object from a non-streaming JSON response.
func extractJSONUsage(body []byte) (usageInfo, bool) {
	var env struct {
		Usage *usageFields `json:"usage"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Usage == nil {
		return usageInfo{}, false
	}
	u := env.Usage.toInfo()
	if u.empty() {
		return usageInfo{}, false
	}
	return u, true
}

// usageFields is the union of provider usage field names.
type usageFields struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
}

func (f usageFields) toInfo() usageInfo {
	u := usageInfo{PromptTokens: f.PromptTokens, CompletionTokens: f.CompletionTokens}
	if f.InputTokens > 0 {
		u.PromptTokens = f.InputTokens
	}
	if f.OutputTokens > 0 {
		u.CompletionTokens = f.OutputTokens
	}
	return u
}

// sseUsageParser incrementally scans an SSE stream for usage, keeping only a
// bounded partial line and the last usage observed.
type sseUsageParser struct {
	partial bytes.Buffer
	info    usageInfo
	found   bool
}

func newSSEUsageParser() *sseUsageParser { return &sseUsageParser{} }

func (p *sseUsageParser) feed(b []byte) {
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			if p.partial.Len()+len(b) <= maxSSELine {
				p.partial.Write(b)
			} else {
				p.partial.Reset()
			}
			return
		}
		if p.partial.Len()+i <= maxSSELine {
			p.partial.Write(b[:i])
			p.handleLine(strings.TrimRight(p.partial.String(), "\r"))
		}
		p.partial.Reset()
		b = b[i+1:]
	}
}

func (p *sseUsageParser) handleLine(line string) {
	if !strings.HasPrefix(line, "data:") {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" || payload == "[DONE]" {
		return
	}
	p.parsePayload(payload)
}

func (p *sseUsageParser) parsePayload(payload string) {
	var env struct {
		// OpenAI-compatible final chunk (and Anthropic message_delta).
		Usage *usageFields `json:"usage"`
		// Anthropic message_start nests usage under "message".
		Message *struct {
			Usage *usageFields `json:"usage"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		return
	}
	if env.Usage != nil {
		if u := env.Usage.toInfo(); !u.empty() {
			p.merge(u)
		}
	}
	if env.Message != nil && env.Message.Usage != nil {
		if u := env.Message.Usage.toInfo(); !u.empty() {
			p.merge(u)
		}
	}
}

// merge keeps the most complete usage seen; later non-zero fields win so a
// message_delta output_tokens updates the earlier message_start input_tokens.
func (p *sseUsageParser) merge(u usageInfo) {
	if u.PromptTokens > 0 {
		p.info.PromptTokens = u.PromptTokens
	}
	if u.CompletionTokens > 0 {
		p.info.CompletionTokens = u.CompletionTokens
	}
	p.found = true
}

func (p *sseUsageParser) usage() (usageInfo, bool) {
	if !p.found || p.info.empty() {
		return usageInfo{}, false
	}
	return p.info, true
}
