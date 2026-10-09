package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// @sk-task responses-streaming#T1.1: mask mapping shared with the streaming relay
const responsesMaskKey = "responses_mask_mapping"

// ResponsesMaskFromContext returns the placeholder→original mapping stored by
// ResponsesShieldMiddleware, or nil when none was produced.
func ResponsesMaskFromContext(c *gin.Context) map[string]string {
	if v, ok := c.Get(responsesMaskKey); ok {
		if m, ok := v.(map[string]string); ok {
			return m
		}
	}
	return nil
}

// @sk-task responses-api#T2.1: reversible shield for the Responses API (AC-003, AC-004, AC-005, AC-006)
//
// ResponsesShieldMiddleware masks the text of an OpenAI Responses request
// (`instructions` and `input`) with the tenant's policy. For non-stream requests
// it installs the buffered unmask writer; for streaming requests it exposes the
// mapping to the streaming relay through the request context.
func ResponsesShieldMiddleware(engine Scanner, cfg *config.ShieldConfig, log *slog.Logger) gin.HandlerFunc {
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
		if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
			abortWithShieldError(c, http.StatusUnsupportedMediaType, "content-type must be application/json", "")
			return
		}
		if len(body) > maxBodySize {
			abortWithShieldError(c, http.StatusRequestEntityTooLarge, "request body too large", "")
			return
		}

		// @sk-task responses-streaming#T1.1: streaming is supported; the mapping
		// is shared with the streaming relay via the request context.
		var meta struct {
			Stream bool `json:"stream"`
		}
		_ = json.Unmarshal(body, &meta)

		tenant, ok := TenantFromContext(c)
		if !ok {
			abortWithShieldError(c, http.StatusBadRequest, "missing tenant in context", "")
			return
		}

		texts, apply, err := responsesFields(body)
		if err != nil {
			c.Header("X-Shield-Status", "error")
			c.AbortWithStatusJSON(http.StatusBadRequest, shieldResponse{ShieldStatus: "error", Error: err.Error()})
			return
		}
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
			log.WarnContext(c.Request.Context(), "responses shield scan failed, applying default_action",
				slog.String("error", scanErr.Error()),
				slog.String("tenant_slug", tenantSlug),
				slog.String("default_action", defaultAction),
			)
			if defaultAction == "block" {
				c.Header("X-Shield-Status", "blocked")
				c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
					ShieldStatus: "blocked",
					Error:        "shield scan unavailable, blocked by default action",
				})
				return
			}
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(apply(masked)))
		// Share the mapping with the streaming relay; for non-stream requests
		// also install the buffered writer that restores placeholders on flush.
		c.Set(responsesMaskKey, mapping)
		if !meta.Stream {
			if w := wrapUnmaskWriter(c, mapping); w != nil {
				defer w.flush()
			}
		}

		duration := time.Since(start)
		scanStatus := string(respStatus(resp))
		metrics.ShieldScanDuration.WithLabelValues(tenantSlug, scanStatus).Observe(float64(duration.Milliseconds()))
		metrics.ShieldProfilesEvaluated.WithLabelValues(tenantSlug).Inc()
		log.InfoContext(c.Request.Context(), "responses shield scan",
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
			c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
				ShieldStatus: "blocked",
				Error:        "request blocked by content shield",
			})
		case value.ScanStatusError:
			abortWithShieldError(c, http.StatusBadGateway, "shield scan error", tenantSlug)
		case value.ScanStatusSuspicious:
			if cfg != nil && cfg.ActionOnSuspicious == "block" {
				c.Header("X-Shield-Status", "blocked")
				publishBlockFacts(c, shieldFindings(resp))
				c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
					ShieldStatus: "blocked",
					Error:        "request blocked by content shield",
				})
				return
			}
			setShieldCleanHeaders(c)
			c.Next()
		default:
			setShieldCleanHeaders(c)
			c.Next()
		}
	}
}

// responsesFields extracts the maskable text of a Responses request and returns
// an apply function that injects the masked texts back into the original body
// while preserving every other field.
func responsesFields(body []byte) ([]string, func([]string) []byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON body")
	}

	var texts []string
	var sets []func(string)
	var finalize []func()

	if inst, ok := raw["instructions"]; ok {
		var s string
		if json.Unmarshal(inst, &s) == nil {
			texts = append(texts, s)
			sets = append(sets, func(m string) {
				if b, e := json.Marshal(m); e == nil {
					raw["instructions"] = b
				}
			})
		}
	}

	if in, ok := raw["input"]; ok {
		var s string
		if json.Unmarshal(in, &s) == nil {
			texts = append(texts, s)
			sets = append(sets, func(m string) {
				if b, e := json.Marshal(m); e == nil {
					raw["input"] = b
				}
			})
		} else {
			var items []json.RawMessage
			if json.Unmarshal(in, &items) == nil {
				for i := range items {
					idx := i
					var istr string
					if json.Unmarshal(items[idx], &istr) == nil {
						texts = append(texts, istr)
						sets = append(sets, func(m string) {
							if b, e := json.Marshal(m); e == nil {
								items[idx] = b
							}
						})
						continue
					}
					var obj map[string]json.RawMessage
					if json.Unmarshal(items[idx], &obj) != nil {
						continue
					}
					collectResponsesItem(obj, &texts, &sets, &finalize)
					finalize = append(finalize, func() {
						if b, e := json.Marshal(obj); e == nil {
							items[idx] = b
						}
					})
				}
				finalize = append(finalize, func() {
					if b, e := json.Marshal(items); e == nil {
						raw["input"] = b
					}
				})
			}
		}
	}

	apply := func(masked []string) []byte {
		for i, set := range sets {
			if i < len(masked) {
				set(masked[i])
			}
		}
		for _, f := range finalize {
			f()
		}
		if b, e := json.Marshal(raw); e == nil {
			return b
		}
		return body
	}
	return texts, apply, nil
}

// collectResponsesItem walks one input item (message object or top-level text
// item) and registers the text slots and container finalizers.
func collectResponsesItem(obj map[string]json.RawMessage, texts *[]string, sets *[]func(string), finalize *[]func()) {
	if t, ok := obj["text"]; ok {
		var s string
		if json.Unmarshal(t, &s) == nil {
			*texts = append(*texts, s)
			*sets = append(*sets, func(m string) {
				if b, e := json.Marshal(m); e == nil {
					obj["text"] = b
				}
			})
		}
	}

	c, ok := obj["content"]
	if !ok {
		return
	}
	var cs string
	if json.Unmarshal(c, &cs) == nil {
		*texts = append(*texts, cs)
		*sets = append(*sets, func(m string) {
			if b, e := json.Marshal(m); e == nil {
				obj["content"] = b
			}
		})
		return
	}

	var blocks []map[string]json.RawMessage
	if json.Unmarshal(c, &blocks) != nil {
		return
	}
	for k := range blocks {
		bi := k
		t, ok := blocks[bi]["text"]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(t, &s) != nil {
			continue
		}
		*texts = append(*texts, s)
		*sets = append(*sets, func(m string) {
			if b, e := json.Marshal(m); e == nil {
				blocks[bi]["text"] = b
			}
		})
	}
	*finalize = append(*finalize, func() {
		if b, e := json.Marshal(blocks); e == nil {
			obj["content"] = b
		}
	})
}
