package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

const maxBodySize = 1 << 20 // 1MB

// @sk-task 13-shield-middleware-wiring#T2.3: Custom ResponseWriter for dict unmask (AC-006)
type dictUnmaskWriter struct {
	gin.ResponseWriter
	buf         bytes.Buffer
	mapping     map[string]string
	statusCode  int
	wroteHeader bool
}

func (w *dictUnmaskWriter) Write(data []byte) (int, error) {
	return w.buf.Write(data)
}

func (w *dictUnmaskWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.statusCode = code
	w.wroteHeader = true
}

func (w *dictUnmaskWriter) flush() {
	if !w.wroteHeader {
		w.statusCode = http.StatusOK
	}
	body := w.buf.String()
	for placeholder, original := range w.mapping {
		body = strings.ReplaceAll(body, placeholder, original)
	}
	w.ResponseWriter.WriteHeader(w.statusCode)
	w.ResponseWriter.Write([]byte(body))
	w.buf.Reset()
}

func wrapUnmaskWriter(c *gin.Context, mapping map[string]string) *dictUnmaskWriter {
	if len(mapping) == 0 {
		return nil
	}
	w := &dictUnmaskWriter{ResponseWriter: c.Writer, mapping: mapping}
	c.Writer = w
	return w
}

// @sk-task 13-shield-middleware-wiring#T3.2: Streaming writer for SSE chunk unmask (AC-007)
type streamDictUnmaskWriter struct {
	gin.ResponseWriter
	mapping map[string]string
}

func (w *streamDictUnmaskWriter) Write(data []byte) (int, error) {
	s := string(data)
	for placeholder, original := range w.mapping {
		s = strings.ReplaceAll(s, placeholder, original)
	}
	return w.ResponseWriter.Write([]byte(s))
}

func wrapStreamUnmaskWriter(c *gin.Context, mapping map[string]string) {
	if len(mapping) == 0 {
		return
	}
	c.Writer = &streamDictUnmaskWriter{ResponseWriter: c.Writer, mapping: mapping}
}

type shieldResponse struct {
	ShieldStatus string `json:"shield_status"`
	Error        string `json:"error,omitempty"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Scanner interface {
	Scan(ctx context.Context, req appshield.ScanRequest) (*appshield.ScanResponse, error)
}

// @sk-task 51-shield-gateway-integration#T2.1: Implement ShieldMiddleware (AC-001, AC-002, AC-003, AC-004, AC-005, AC-006)
// @sk-task 61-observability#T3.1: Instrument shield middleware with span attributes and metrics (AC-004)
// @sk-task tenant-profile-sync#T3.1: ShieldMiddleware reads dictionaries from tenant (AC-006, AC-007)
// @sk-task remove-audit-incidents#T2.3: Remove incident creation, X-Shield-Incident-ID, and ShieldIncidentsBySeverity (AC-008, AC-011)
func ShieldMiddleware(engine Scanner, cfg *config.ShieldConfig, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		body, err := c.GetRawData()
		if err != nil {
			log.Error("failed to read request body", zap.Error(err))
			abortWithShieldError(c, http.StatusInternalServerError, "failed to read body", "")
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		if len(body) == 0 {
			c.Next()
			return
		}

		if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
			abortWithShieldError(c, http.StatusUnsupportedMediaType, "content-type must be application/json", "")
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

		var chatReq chatRequest
		if err := json.Unmarshal(body, &chatReq); err != nil {
			abortWithShieldError(c, http.StatusBadRequest, "invalid JSON body", tenant.Slug().String())
			return
		}

		promptText := extractPromptText(chatReq.Messages)
		if promptText == "" {
			setShieldCleanHeaders(c)
			c.Next()
			return
		}

		tenantSlug := tenant.Slug().String()
		piiCfg := tenant.PIIConfig()

		dictDetectors := make([]*detector.DictionaryDetector, 0, len(tenant.Dictionaries()))
		for _, dict := range tenant.Dictionaries() {
			dictDetectors = append(dictDetectors, detector.NewDictionaryDetector(dict))
		}

		hasDictHits := false
		for _, dd := range dictDetectors {
			results, err := dd.Scan(c.Request.Context(), promptText)
			if err != nil || len(results) == 0 {
				continue
			}
			hasDictHits = true
		}

		// @sk-task 13-shield-middleware-wiring#T2.2: Store dict mask mapping per-request for later unmask (AC-006)
		dictMaskMapping := make(map[string]string)
		if hasDictHits {
			dictMaskID := mask.NewShortID()
			phCounter := 0
			for mi, msg := range chatReq.Messages {
				if msg.Content == "" {
					continue
				}
				for _, dd := range dictDetectors {
					results, _ := dd.Scan(c.Request.Context(), msg.Content)
					if len(results) == 0 {
						continue
					}
					sort.Slice(results, func(i, j int) bool {
						return results[i].StartPos > results[j].StartPos
					})
					for _, r := range results {
						ph := fmt.Sprintf("{{dict.%s.%d}}", dictMaskID, phCounter)
						dictMaskMapping[ph] = r.Fragment
						phCounter++
						chatReq.Messages[mi].Content = chatReq.Messages[mi].Content[:r.StartPos] + ph + chatReq.Messages[mi].Content[r.StartPos+len(r.Fragment):]
					}
				}
			}
			if phCounter > 0 {
				newBody, _ := json.Marshal(chatReq)
				c.Request.Body = io.NopCloser(bytes.NewBuffer(newBody))
				c.Header("X-Shield-Dict-Mask-ID", dictMaskID)
			}
		}

		// @sk-task 13-shield-middleware-wiring#T2.1: Read PIIConfig from tenant, scan with Rules (AC-001, AC-003, AC-007)
		var resp *appshield.ScanResponse
		// Dictionaries have priority: re-extract promptText after dict masking
		// so PII scan sees already-masked text (no false positives on known terms)
		piiText := extractPromptText(chatReq.Messages)
		if engine != nil && piiCfg.Enabled && len(piiCfg.Rules) > 0 {
			resp, err = engine.Scan(c.Request.Context(), appshield.ScanRequest{
				Text:  piiText,

// @sk-task 13-shield-middleware-wiring#T4.2: After dict masking, re-extract text for PII scan
				Rules: piiCfg.Rules,
			})
			// @sk-task 13-shield-middleware-wiring#T2.1: Graceful degradation on engine.Scan error (AC-004)
			if err != nil {
				defaultAction := piiCfg.DefaultAction
				if defaultAction == "" {
					defaultAction = "block"
				}
				log.Warn("engine scan failed, applying default_action",
					zap.Error(err),
					zap.String("tenant_slug", tenantSlug),
					zap.String("default_action", defaultAction),
				)
				if defaultAction == "block" {
					c.Header("X-Shield-Status", "blocked")
					c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
						ShieldStatus: "blocked",
						Error:        "shield scan unavailable, blocked by default action",
					})
					return
				}
				if chatReq.Stream {
					wrapStreamUnmaskWriter(c, dictMaskMapping)
				} else if w := wrapUnmaskWriter(c, dictMaskMapping); w != nil {
					defer w.flush()
				}
				c.Next()
				return
			}
		}

		duration := time.Since(start)

		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(
			attribute.String("shield.tenant", tenantSlug),
			attribute.String("shield.status", string(respStatus(resp))),
		)

		scanStatus := string(respStatus(resp))
		metrics.ShieldScanDuration.WithLabelValues(tenantSlug, scanStatus).Observe(float64(duration.Milliseconds()))
		metrics.ShieldProfilesEvaluated.WithLabelValues(tenantSlug).Inc()

		// @sk-task 13-shield-middleware-wiring#T3.3: Log pii_enabled, rules_count and unmasked fields
		unmasked := len(dictMaskMapping) > 0
		log.Info("shield scan",
			zap.String("shield_status", scanStatus),
			zap.String("tenant_slug", tenantSlug),
			zap.Bool("pii_enabled", piiCfg.Enabled),
			zap.Int("rules_count", len(piiCfg.Rules)),
			zap.String("model", chatReq.Model),
			zap.Duration("latency", duration),
			zap.Bool("unmasked", unmasked),
		)

		status := respStatus(resp)
		switch status {
		case value.ScanStatusBlocked:
			c.Header("X-Shield-Status", "blocked")
			c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
				ShieldStatus: "blocked",
				Error:        "request blocked by content shield",
			})

		case value.ScanStatusError:
			abortWithShieldError(c, http.StatusBadGateway, "shield scan error", tenantSlug)

		case value.ScanStatusSuspicious:
			if cfg != nil && cfg.ActionOnSuspicious == "block" {
				c.Header("X-Shield-Status", "blocked")
				c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
					ShieldStatus: "blocked",
					Error:        "request blocked by content shield",
				})
			} else {
				setShieldCleanHeaders(c)
				if chatReq.Stream {
					wrapStreamUnmaskWriter(c, dictMaskMapping)
				} else if w := wrapUnmaskWriter(c, dictMaskMapping); w != nil {
					defer w.flush()
				}
				c.Next()
			}

		default:
			setShieldCleanHeaders(c)
			if chatReq.Stream {
				wrapStreamUnmaskWriter(c, dictMaskMapping)
			} else if w := wrapUnmaskWriter(c, dictMaskMapping); w != nil {
				defer w.flush()
			}
			c.Next()
		}
	}
}

func respStatus(resp *appshield.ScanResponse) value.ScanStatus {
	if resp != nil {
		return resp.Status()
	}
	return value.ScanStatusClean
}

func extractPromptText(messages []chatMessage) string {
	var texts []string
	for _, msg := range messages {
		if msg.Content != "" {
			texts = append(texts, msg.Content)
		}
	}
	return strings.Join(texts, "\n")
}

func setShieldCleanHeaders(c *gin.Context) {
	c.Header("X-Shield-Status", "clean")
}

func abortWithShieldError(c *gin.Context, status int, message string, _ string) {
	c.Header("X-Shield-Status", "error")
	c.AbortWithStatusJSON(status, shieldResponse{
		ShieldStatus: "error",
		Error:        message,
	})
}
