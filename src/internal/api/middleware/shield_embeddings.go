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

// @sk-task embeddings-passthrough#T1.1: embeddings input shield (AC-002, AC-003, AC-004)
//
// embeddingsRequest is the OpenAI-compatible embeddings body shape.
type embeddingsRequest struct {
	Model string          `json:"model"`
	Input json.RawMessage `json:"input"`
}

// EmbeddingsShieldMiddleware masks the text input of an embeddings request using
// the tenant's existing policy (dictionaries + PII rules) before the provider is
// called. Unlike the chat path it never unmasks: embeddings vectors cannot be
// reversed, so masking is one-way and only the request is rewritten.
func EmbeddingsShieldMiddleware(engine Scanner, cfg *config.ShieldConfig, log *slog.Logger) gin.HandlerFunc {
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

		var req embeddingsRequest
		if err := json.Unmarshal(body, &req); err != nil {
			abortWithShieldError(c, http.StatusBadRequest, "invalid JSON body", tenant.Slug().String())
			return
		}

		texts, isArray, err := parseEmbeddingsInput(req.Input)
		if err != nil {
			// Non-text input (for example token arrays) cannot be masked, so it is
			// rejected rather than passed through and leaked.
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

		// @sk-task openai-endpoint-coverage#T2.1: shared dictionary+PII masking core (AC-002)
		masked, phCounter, resp, scanErr := maskTexts(c.Request.Context(), engine, tenant, texts)
		if scanErr != nil {
			defaultAction := piiCfg.DefaultAction
			if defaultAction == "" {
				defaultAction = "block"
			}
			log.WarnContext(c.Request.Context(), "embeddings shield scan failed, applying default_action",
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

		c.Request.Body = io.NopCloser(bytes.NewBuffer(marshalEmbeddingsBody(body, masked, isArray)))

		duration := time.Since(start)
		scanStatus := string(respStatus(resp))
		metrics.ShieldScanDuration.WithLabelValues(tenantSlug, scanStatus).Observe(float64(duration.Milliseconds()))
		metrics.ShieldProfilesEvaluated.WithLabelValues(tenantSlug).Inc()
		log.InfoContext(c.Request.Context(), "embeddings shield scan",
			slog.String("shield_status", scanStatus),
			slog.String("tenant_slug", tenantSlug),
			slog.Bool("pii_enabled", piiCfg.Enabled),
			slog.Int("rules_count", len(piiCfg.Rules)),
			slog.Int("dict_masked", phCounter),
			slog.Int("pii_masked", piiMaskedCount(resp)),
			slog.String("model", req.Model),
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

// parseEmbeddingsInput accepts a string or an array of strings and rejects any
// other shape (token arrays, base64, numbers) because those cannot be masked.
func parseEmbeddingsInput(raw json.RawMessage) (texts []string, isArray bool, err error) {
	if len(raw) == 0 {
		return nil, false, fmt.Errorf("input is required")
	}
	var single string
	if json.Unmarshal(raw, &single) == nil {
		if single == "" {
			return nil, false, fmt.Errorf("input must not be empty")
		}
		return []string{single}, false, nil
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		if len(arr) == 0 {
			return nil, true, fmt.Errorf("input must not be empty")
		}
		for _, t := range arr {
			if t == "" {
				return nil, true, fmt.Errorf("input elements must not be empty")
			}
		}
		return arr, true, nil
	}
	return nil, false, fmt.Errorf("unsupported input: expected a string or an array of strings")
}

// marshalEmbeddingsBody rewrites only the input field, preserving every other
// request field (model, dimensions, encoding_format, user) untouched.
func marshalEmbeddingsBody(original []byte, texts []string, isArray bool) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(original, &raw); err != nil {
		return original
	}
	if isArray {
		if b, err := json.Marshal(texts); err == nil {
			raw["input"] = b
		}
	} else if len(texts) == 1 {
		if b, err := json.Marshal(texts[0]); err == nil {
			raw["input"] = b
		}
	}
	out, err := json.Marshal(raw)
	if err != nil {
		return original
	}
	return out
}
