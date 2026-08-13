package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	conversationapp "github.com/bzdvdn/maskchain/src/internal/app/conversation"
	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

const conversationMaxBodySize = 1 << 20 // 1MB

// @sk-task conversation-logging#T2.2: Implement ConversationMiddleware capturing raw chat exchanges (RQ-008, AC-001, AC-010, AC-011, DEC-001, DEC-002)
//
// ConversationMiddleware captures the original request, the final response as
// seen by the client, and the shield mask mapping, then enqueues a raw record
// for the background worker. It runs before the shield middleware, so the
// captured request is the unmodified original. Encryption and persistence are
// delegated to the worker; the hot path only reads/restores the body, buffers
// the response and enqueues. Retention mode gating (none/meta/full) happens at
// this capture boundary so sensitive content never leaves the middleware.
type ConversationMiddleware struct {
	worker conversationapp.Sender
	log    *slog.Logger
	// defaultRetentionMode is the global data.retention.mode default, applied
	// when the tenant in context has no explicit retention mode (DEC-002).
	defaultRetentionMode value.RetentionMode
}

// @sk-task conversation-logging#T2.2: NewConversationMiddleware creates the middleware (RQ-008, DEC-001)
func NewConversationMiddleware(sender conversationapp.Sender, log *slog.Logger) *ConversationMiddleware {
	return &ConversationMiddleware{
		worker:               sender,
		log:                  log,
		defaultRetentionMode: value.RetentionModeFull,
	}
}

// SetDefaultRetentionMode sets the global retention default (data.retention.mode).
func (m *ConversationMiddleware) SetDefaultRetentionMode(mode value.RetentionMode) {
	if mode.Valid() {
		m.defaultRetentionMode = mode
	}
}

// effectiveRetentionMode resolves the mode for the tenant in context: its
// explicit value when set, otherwise the global default (DEC-002).
func (m *ConversationMiddleware) effectiveRetentionMode(c *gin.Context) value.RetentionMode {
	if t, ok := TenantFromContext(c); ok && t.RetentionMode() != "" {
		return t.RetentionMode()
	}
	return m.defaultRetentionMode
}

// @sk-task conversation-logging#T2.5: Buffer the response with a 1MB cap without stalling the client stream (RQ-009, DEC-007, AC-003)
//
// bufferedWriter buffers the response body while forwarding it to the client.
// The internal buffer is capped at conversationMaxBodySize; once full, writes
// are still forwarded to the client but no longer accumulated.
type bufferedWriter struct {
	gin.ResponseWriter
	buf     bytes.Buffer
	written bool
	full    bool
}

func (w *bufferedWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	if !w.full {
		room := conversationMaxBodySize - w.buf.Len()
		if room > 0 {
			n := len(data)
			if n > room {
				n = room
			}
			w.buf.Write(data[:n])
		}
		if w.buf.Len() >= conversationMaxBodySize {
			w.full = true
		}
	}
	return w.ResponseWriter.Write(data)
}

func (w *bufferedWriter) WriteString(s string) (int, error) {
	if !w.written {
		w.written = true
	}
	if !w.full {
		room := conversationMaxBodySize - w.buf.Len()
		if room > 0 {
			n := len(s)
			if n > room {
				n = room
			}
			w.buf.WriteString(s[:n])
		}
		if w.buf.Len() >= conversationMaxBodySize {
			w.full = true
		}
	}
	return w.ResponseWriter.WriteString(s)
}

// @sk-task conversation-logging#T2.2: Handler runs the capture pipeline (RQ-008, AC-001, AC-010, AC-011)
// @sk-task conversation-logging#T2.5: Capture streaming SSE responses verbatim (RQ-009, DEC-007, AC-003)
func (m *ConversationMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		body, err := c.GetRawData()
		if err != nil {
			m.log.WarnContext(c.Request.Context(), "conversation middleware: failed to read body",
				slog.String("error", err.Error()))
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		if len(body) == 0 || len(body) > conversationMaxBodySize {
			c.Next()
			return
		}

		var chatReq struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.Unmarshal(body, &chatReq); err != nil || chatReq.Model == "" {
			c.Next()
			return
		}

		buf := &bufferedWriter{ResponseWriter: c.Writer}
		c.Writer = buf
		start := time.Now()

		c.Next()

		tenantSlug := extractTenantSlugForConversation(c)
		status := conversationStatusFrom(c, buf.written && c.Writer.Status() >= 500)
		if chatReq.Stream && status == conversation.StatusOK && !bytes.Contains(buf.buf.Bytes(), []byte("[DONE]")) {
			status = conversation.StatusError
		}

		masking := maskingFromContext(c)
		mode := m.effectiveRetentionMode(c)

		// DEC-003: gating at the capture boundary. In `none` no RawRecord is
		// ever produced; session/analytics counters are untouched (AC-003).
		if mode == value.RetentionModeNone {
			return
		}

		detector, category := blockFactsFromContext(c)

		// DEC-005: log all chat requests in full/meta so the masked/not-masked
		// filter is meaningful (AC-008).
		if mode == value.RetentionModeMeta {
			rec := conversationapp.RawRecord{
				ID:        conversationRecordID(c),
				TenantID:  tenantSlug,
				Model:     chatReq.Model,
				Status:    status,
				Streamed:  chatReq.Stream,
				MaskID:    conversationMaskIDFromContext(c),
				Detector:  detector,
				Category:  category,
				CreatedAt: start.UTC(),
			}
			m.worker.Send(rec)
			return
		}

		rec := conversationapp.RawRecord{
			ID:        conversationRecordID(c),
			TenantID:  tenantSlug,
			Model:     chatReq.Model,
			Status:    status,
			Streamed:  chatReq.Stream,
			MaskID:    conversationMaskIDFromContext(c),
			Detector:  detector,
			Category:  category,
			Request:   append([]byte(nil), body...),
			Response:  append([]byte(nil), buf.buf.Bytes()...),
			Masking:   masking,
			CreatedAt: start.UTC(),
		}

		m.worker.Send(rec)
	}
}

func blockFactsFromContext(c *gin.Context) (detector, category string) {
	if v, ok := c.Get(conversationDetectorKey); ok {
		if s, ok := v.(string); ok {
			detector = s
		}
	}
	if v, ok := c.Get(conversationCategoryKey); ok {
		if s, ok := v.(string); ok {
			category = s
		}
	}
	return detector, category
}

func conversationStatusFrom(c *gin.Context, upstreamError bool) conversation.ConversationStatus {
	if upstreamError {
		return conversation.StatusError
	}
	switch c.Writer.Header().Get("X-Shield-Status") {
	case "blocked":
		return conversation.StatusBlocked
	case "error":
		return conversation.StatusError
	default:
		return conversation.StatusOK
	}
}

func maskingFromContext(c *gin.Context) []conversation.MaskingEntry {
	v, ok := c.Get(conversationMaskKey)
	if !ok {
		return nil
	}
	mapping, ok := v.(map[string]string)
	if !ok || len(mapping) == 0 {
		return nil
	}
	entries := make([]conversation.MaskingEntry, 0, len(mapping))
	for ph, original := range mapping {
		entries = append(entries, conversation.MaskingEntry{Placeholder: ph, Original: original})
	}
	return entries
}

func conversationRecordID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if id, ok := v.(string); ok && id != "" {
			return id
		}
	}
	return uuid.New().String()
}

// @sk-task conversation-logging#T5.1: Read the shield dict mask id from context (AC-005)
func conversationMaskIDFromContext(c *gin.Context) string {
	v, ok := c.Get(conversationMaskIDKey)
	if !ok {
		return ""
	}
	if id, ok := v.(string); ok {
		return id
	}
	return ""
}

func extractTenantSlugForConversation(c *gin.Context) string {
	if t, ok := TenantFromContext(c); ok {
		return t.Slug().String()
	}
	if slug := c.GetHeader("X-Tenant-ID"); slug != "" {
		return slug
	}
	return "unknown"
}
