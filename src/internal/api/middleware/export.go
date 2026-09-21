package middleware

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// maxExportBody bounds the captured response body so a large response cannot
// grow memory without limit; content is best-effort beyond this size.
const maxExportBody = 1 << 20 // 1 MiB

// ExportRecorder accepts captured records for asynchronous delivery.
type ExportRecorder interface {
	Enqueue(record domainlogexport.Record)
}

type exportWriter struct {
	gin.ResponseWriter
	buf       bytes.Buffer
	status    int
	truncated bool
}

func (w *exportWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *exportWriter) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *exportWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *exportWriter) capture(b []byte) {
	if w.truncated {
		return
	}
	if w.buf.Len()+len(b) > maxExportBody {
		w.truncated = true
		return
	}
	w.buf.Write(b)
}

// @sk-task log-export#T2.2: capture masked request/response for export (AC-001, AC-002, AC-004)
//
// ExportMiddleware runs after the shield (so the request body is already masked
// and the response writer sees the provider's masked bytes) and before the
// provider handler. It records only masked content and metadata, applies the
// tenant retention mode, and never alters the client response.
func ExportMiddleware(recorder ExportRecorder, rates *analytics.CostRateRegistry, log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}
	return func(c *gin.Context) {
		if recorder == nil || c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		tenant, ok := TenantFromContext(c)
		if !ok || tenant == nil {
			c.Next()
			return
		}

		mode := tenant.RetentionMode()
		// Zero-retention tenants never export anything.
		if mode == value.RetentionModeNone {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		w := &exportWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = w
		c.Next()

		var req struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &req)

		tokens, cost := exportUsage(w.buf.Bytes(), req.Model, rates)
		rec := domainlogexport.Record{
			EventID:   newExportEventID(),
			Timestamp: time.Now().UTC(),
			Tenant:    tenant.Slug().String(),
			Model:     req.Model,
			Status:    http.StatusText(w.status),
			Tokens:    tokens,
			Cost:      cost,
		}
		if v, ok := c.Get(conversationMaskIDKey); ok {
			if s, ok := v.(string); ok {
				rec.MaskID = s
			}
		}
		// Retention mode "meta" exports metadata only; "full" (or unset) also
		// exports masked content.
		if mode == value.RetentionModeFull || mode == "" {
			rec.Content = &domainlogexport.Content{
				MaskedRequest:  string(body),
				MaskedResponse: w.buf.String(),
			}
		}
		recorder.Enqueue(rec)
	}
}

func exportUsage(body []byte, model string, rates *analytics.CostRateRegistry) (tokens int64, cost float64) {
	var resp struct {
		Usage *struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Usage == nil {
		return 0, 0
	}
	tokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
	if rates != nil && model != "" {
		rate, _ := rates.Resolve(model)
		cost = rate.Cost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}
	return tokens, cost
}

func newExportEventID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
