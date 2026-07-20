package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
)

const (
	SkipEnvelopeKey = "skipEnvelope"
	EnvelopedKey    = "enveloped"
)

// @sk-task 118-api-consistency#T1.2: ResponseEnvelope middleware wrapping JSON responses (AC-003, AC-004, AC-010)
//
// ResponseEnvelope handles the operation.
func ResponseEnvelope() gin.HandlerFunc {
	return func(c *gin.Context) {
		w := &envelopeWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = w
		c.Next()

		w.Header().Del("Content-Length")

		if c.GetBool(SkipEnvelopeKey) {
			w.ResponseWriter.WriteHeader(w.status)
			w.ResponseWriter.Write(w.body.Bytes())
			return
		}

		// Exclude health, metrics, and debug paths from envelope (spec: out of scope)
		path := c.Request.URL.Path
		if path == "/health" || path == "/ready" || path == "/live" || path == "/metrics" || strings.HasPrefix(path, "/debug/") {
			w.ResponseWriter.WriteHeader(w.status)
			w.ResponseWriter.Write(w.body.Bytes())
			return
		}

		if w.status == http.StatusNoContent {
			w.ResponseWriter.WriteHeader(w.status)
			return
		}

		if c.GetBool(EnvelopedKey) {
			w.ResponseWriter.WriteHeader(w.status)
			w.ResponseWriter.Write(w.body.Bytes())
			return
		}

		ct := w.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") || w.body.Len() == 0 {
			w.ResponseWriter.WriteHeader(w.status)
			w.ResponseWriter.Write(w.body.Bytes())
			return
		}

		var payload any
		json.Unmarshal(w.body.Bytes(), &payload)

		var resp dto.ApiResponse
		if w.status >= 400 {
			var legacy struct {
				Error string `json:"error"`
				Code  string `json:"code"`
			}
			if json.Unmarshal(w.body.Bytes(), &legacy) == nil && legacy.Error != "" {
				code := legacy.Code
				if code == "" {
					code = "INTERNAL_ERROR"
				}
				resp = dto.NewErrorResponse(code, legacy.Error)
			} else {
				resp = dto.NewErrorResponse("INTERNAL_ERROR", http.StatusText(w.status))
			}
		} else {
			resp = dto.NewSuccessResponse(payload)
			if p, exists := c.Get("pagination"); exists {
				if pag, ok := p.(dto.Pagination); ok {
					resp.Pagination = &pag
				}
			}
		}

		w.Header().Del("Content-Length")
		w.ResponseWriter.WriteHeader(w.status)
		json.NewEncoder(w.ResponseWriter).Encode(resp)
	}
}

type envelopeWriter struct {
	gin.ResponseWriter
	body    bytes.Buffer
	status  int
	written bool
}

func (w *envelopeWriter) WriteHeader(status int) {
	w.status = status
}

func (w *envelopeWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *envelopeWriter) WriteString(s string) (int, error) {
	return w.body.WriteString(s)
}

func (w *envelopeWriter) Status() int {
	return w.status
}

func (w *envelopeWriter) Written() bool {
	return w.written
}

func (w *envelopeWriter) Size() int {
	return w.body.Len()
}

func (w *envelopeWriter) WriteHeaderNow() {
	if !w.written {
		w.written = true
		w.ResponseWriter.WriteHeader(w.status)
	}
}
