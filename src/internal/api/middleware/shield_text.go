package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// @sk-task openai-endpoint-coverage#T2.1: shared non-chat masking core (AC-003, AC-004, AC-005)
//
// maskTexts applies dictionary masking followed by the tenant PII scan to a set
// of text fields, returning the rewritten texts, the number of dictionary
// placeholders, and the scan response. It is the shared core of the non-chat
// input shields (embeddings, moderations, rerank, count_tokens). Masking is
// one-way: callers never unmask provider output.
func maskTexts(ctx context.Context, engine Scanner, tenant *entity.Tenant, texts []string) ([]string, int, *appshield.ScanResponse, error) {
	out := make([]string, len(texts))
	copy(out, texts)

	piiCfg := tenant.PIIConfig()

	// Dictionary masking first, mirroring the chat path: known business terms
	// become [MASK.N] placeholders so the PII scan does not re-flag them.
	dictDetectors := make([]*detector.DictionaryDetector, 0, len(tenant.Dictionaries()))
	for _, dict := range tenant.Dictionaries() {
		if dict == nil {
			continue
		}
		dictDetectors = append(dictDetectors, detector.NewDictionaryDetector(dict))
	}
	phCounter := 0
	for i, text := range out {
		var all []detector.DetectorResult
		for _, dd := range dictDetectors {
			results, scanErr := dd.Scan(ctx, text)
			if scanErr != nil {
				continue
			}
			all = append(all, results...)
		}
		if len(all) == 0 {
			continue
		}
		kept := mask.ResolveOverlaps(all)
		sort.Slice(kept, func(a, b int) bool { return kept[a].StartPos > kept[b].StartPos })
		for _, r := range kept {
			ph := fmt.Sprintf("[MASK.%d]", phCounter)
			phCounter++
			out[i] = out[i][:r.StartPos] + ph + out[i][r.StartPos+len(r.Fragment):]
		}
	}

	if engine == nil || !piiCfg.Enabled || len(piiCfg.Rules) == 0 {
		return out, phCounter, nil, nil
	}

	resp, err := engine.Scan(ctx, appshield.ScanRequest{
		Text:  strings.Join(out, "\n"),
		Rules: piiCfg.Rules,
	})
	if err != nil {
		return out, phCounter, nil, err
	}
	if resp != nil && len(resp.Replacements) > 0 {
		for i := range out {
			for ph, original := range resp.Replacements {
				out[i] = strings.ReplaceAll(out[i], original, ph)
			}
		}
	}
	return out, phCounter, resp, nil
}

// @sk-task openai-endpoint-coverage#T2.2: generic non-chat input shield (AC-003, AC-004, AC-005)
//
// InputKind selects which endpoint body shape the middleware masks.
type InputKind string

const (
	InputModerations InputKind = "moderations"
	InputRerank      InputKind = "rerank"
	InputCountTokens InputKind = "count_tokens"
)

// TextInputShieldMiddleware masks the text fields of a non-chat endpoint
// (moderations, rerank, Anthropic count_tokens) using the tenant's existing
// policy before the provider is called. Like the embeddings shield it never
// unmasks: only the request is rewritten.
func TextInputShieldMiddleware(kind InputKind, engine Scanner, cfg *config.ShieldConfig, log *slog.Logger) gin.HandlerFunc {
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

		tenant, ok := TenantFromContext(c)
		if !ok {
			abortWithShieldError(c, http.StatusBadRequest, "missing tenant in context", "")
			return
		}

		fields, err := textFieldsFor(kind, body)
		if err != nil {
			// The shape cannot be masked safely, so reject rather than risk a leak.
			c.Header("X-Shield-Status", "error")
			c.AbortWithStatusJSON(http.StatusBadRequest, shieldResponse{
				ShieldStatus: "error",
				Error:        err.Error(),
			})
			return
		}

		tenantSlug := tenant.Slug().String()
		piiCfg := tenant.PIIConfig()
		start := time.Now()

		masked, dictMasked, resp, scanErr := maskTexts(c.Request.Context(), engine, tenant, fields.Texts)
		if scanErr != nil {
			defaultAction := piiCfg.DefaultAction
			if defaultAction == "" {
				defaultAction = "block"
			}
			log.WarnContext(c.Request.Context(), "text shield scan failed, applying default_action",
				slog.String("error", scanErr.Error()),
				slog.String("tenant_slug", tenantSlug),
				slog.String("kind", string(kind)),
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

		c.Request.Body = io.NopCloser(bytes.NewBuffer(fields.Rewrite(masked)))

		duration := time.Since(start)
		scanStatus := string(respStatus(resp))
		metrics.ShieldScanDuration.WithLabelValues(tenantSlug, scanStatus).Observe(float64(duration.Milliseconds()))
		metrics.ShieldProfilesEvaluated.WithLabelValues(tenantSlug).Inc()
		log.InfoContext(c.Request.Context(), "text shield scan",
			slog.String("kind", string(kind)),
			slog.String("shield_status", scanStatus),
			slog.String("tenant_slug", tenantSlug),
			slog.Bool("pii_enabled", piiCfg.Enabled),
			slog.Int("rules_count", len(piiCfg.Rules)),
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

// textFields is the result of extracting the maskable fields from a request
// body: the texts in order, and a rewrite that injects the masked texts back.
type textFields struct {
	Texts   []string
	Rewrite func(masked []string) []byte
}

func textFieldsFor(kind InputKind, body []byte) (textFields, error) {
	switch kind {
	case InputModerations:
		return moderationsFields(body)
	case InputRerank:
		return rerankFields(body)
	case InputCountTokens:
		return countTokensFields(body)
	default:
		return textFields{}, fmt.Errorf("unsupported input kind %q", kind)
	}
}

// moderationsFields extracts the OpenAI moderations "input", which is a string
// or an array of strings.
func moderationsFields(body []byte) (textFields, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return textFields{}, fmt.Errorf("invalid JSON body")
	}
	rawInput, ok := raw["input"]
	if !ok {
		return textFields{}, fmt.Errorf("input is required")
	}

	var single string
	if json.Unmarshal(rawInput, &single) == nil {
		return textFields{
			Texts: []string{single},
			Rewrite: func(masked []string) []byte {
				if len(masked) > 0 {
					if b, err := json.Marshal(masked[0]); err == nil {
						raw["input"] = b
					}
				}
				return marshalRaw(raw, body)
			},
		}, nil
	}

	var arr []string
	if json.Unmarshal(rawInput, &arr) == nil {
		return textFields{
			Texts: append([]string(nil), arr...),
			Rewrite: func(masked []string) []byte {
				if b, err := json.Marshal(masked); err == nil {
					raw["input"] = b
				}
				return marshalRaw(raw, body)
			},
		}, nil
	}

	return textFields{}, fmt.Errorf("unsupported input: expected a string or an array of strings")
}

// rerankFields extracts "query" and every "documents[].text" (documents may also
// be a plain array of strings).
func rerankFields(body []byte) (textFields, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return textFields{}, fmt.Errorf("invalid JSON body")
	}

	var texts []string
	var setters []func(string)

	if q, ok := raw["query"]; ok {
		var qs string
		if json.Unmarshal(q, &qs) == nil {
			texts = append(texts, qs)
			setters = append(setters, func(m string) {
				if b, err := json.Marshal(m); err == nil {
					raw["query"] = b
				}
			})
		}
	}

	var docs []json.RawMessage
	if d, ok := raw["documents"]; ok {
		_ = json.Unmarshal(d, &docs)
	}
	for i := range docs {
		idx := i
		var s string
		if json.Unmarshal(docs[idx], &s) == nil {
			texts = append(texts, s)
			setters = append(setters, func(m string) {
				if b, err := json.Marshal(m); err == nil {
					docs[idx] = b
				}
			})
			continue
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(docs[idx], &obj) != nil {
			continue
		}
		if t, ok := obj["text"]; ok {
			var ts string
			if json.Unmarshal(t, &ts) != nil {
				continue
			}
			texts = append(texts, ts)
			setters = append(setters, func(m string) {
				if b, err := json.Marshal(m); err == nil {
					obj["text"] = b
					docs[idx], _ = json.Marshal(obj)
				}
			})
		}
	}

	return textFields{
		Texts: texts,
		Rewrite: func(masked []string) []byte {
			for i, set := range setters {
				if i < len(masked) {
					set(masked[i])
				}
			}
			if len(docs) > 0 {
				if b, err := json.Marshal(docs); err == nil {
					raw["documents"] = b
				}
			}
			return marshalRaw(raw, body)
		},
	}, nil
}

// contentValue is an Anthropic content field: either a string or an array of
// blocks, each optionally carrying a "text" string.
type contentValue struct {
	isString bool
	str      string
	blocks   []map[string]json.RawMessage
}

func parseContent(raw json.RawMessage) (*contentValue, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return &contentValue{isString: true, str: s}, true
	}
	var blocks []map[string]json.RawMessage
	if json.Unmarshal(raw, &blocks) == nil {
		return &contentValue{blocks: blocks}, true
	}
	return nil, false
}

func (cv *contentValue) textCount() int {
	if cv.isString {
		return 1
	}
	n := 0
	for i := range cv.blocks {
		if _, ok := cv.blocks[i]["text"]; ok {
			n++
		}
	}
	return n
}

func (cv *contentValue) collect(texts []string) []string {
	if cv.isString {
		return append(texts, cv.str)
	}
	for i := range cv.blocks {
		if t, ok := cv.blocks[i]["text"]; ok {
			var ts string
			if json.Unmarshal(t, &ts) == nil {
				texts = append(texts, ts)
			}
		}
	}
	return texts
}

func (cv *contentValue) apply(masked []string) json.RawMessage {
	if cv.isString {
		if len(masked) > 0 {
			if b, err := json.Marshal(masked[0]); err == nil {
				return b
			}
		}
		if b, err := json.Marshal(cv.str); err == nil {
			return b
		}
		return nil
	}
	k := 0
	for i := range cv.blocks {
		if _, ok := cv.blocks[i]["text"]; ok {
			if k < len(masked) {
				if b, err := json.Marshal(masked[k]); err == nil {
					cv.blocks[i]["text"] = b
				}
			}
			k++
		}
	}
	b, _ := json.Marshal(cv.blocks)
	return b
}

// countTokensFields extracts the text of an Anthropic count_tokens request:
// the "system" field and each "messages[].content", supporting both the string
// and the content-block array shapes.
func countTokensFields(body []byte) (textFields, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return textFields{}, fmt.Errorf("invalid JSON body")
	}

	var texts []string

	var systemCV *contentValue
	if s, ok := raw["system"]; ok {
		if cv, ok := parseContent(s); ok {
			systemCV = cv
			texts = cv.collect(texts)
		}
	}

	var messages []map[string]json.RawMessage
	if m, ok := raw["messages"]; ok {
		_ = json.Unmarshal(m, &messages)
	}
	messageCV := make([]*contentValue, len(messages))
	for i := range messages {
		if c, ok := messages[i]["content"]; ok {
			if cv, ok := parseContent(c); ok {
				messageCV[i] = cv
				texts = cv.collect(texts)
			}
		}
	}

	return textFields{
		Texts: texts,
		Rewrite: func(masked []string) []byte {
			pos := 0
			take := func(n int) []string {
				end := pos + n
				if end > len(masked) {
					end = len(masked)
				}
				if pos > end {
					pos = end
				}
				s := masked[pos:end]
				pos = end
				return s
			}
			if systemCV != nil {
				raw["system"] = systemCV.apply(take(systemCV.textCount()))
			}
			for i := range messages {
				if messageCV[i] != nil {
					messages[i]["content"] = messageCV[i].apply(take(messageCV[i].textCount()))
				}
			}
			if len(messages) > 0 {
				if b, err := json.Marshal(messages); err == nil {
					raw["messages"] = b
				}
			}
			return marshalRaw(raw, body)
		},
	}, nil
}

// marshalRaw re-encodes a raw-field map, falling back to the original body if
// encoding fails so a request is never silently corrupted.
func marshalRaw(raw map[string]json.RawMessage, original []byte) []byte {
	out, err := json.Marshal(raw)
	if err != nil {
		return original
	}
	return out
}
