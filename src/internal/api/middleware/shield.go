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
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

const maxBodySize = 1 << 20 // 1MB

type shieldResponse struct {
	ShieldStatus string `json:"shield_status"`
	IncidentID   string `json:"incident_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
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
			setShieldCleanHeaders(c, "")
			c.Next()
			return
		}

		tenantSlug := tenant.Slug().String()

		var incidents []entity.Incident

		for _, dict := range tenant.Dictionaries() {
			dd := detector.NewDictionaryDetector(dict)
			results, err := dd.Scan(c.Request.Context(), promptText)
			if err != nil || len(results) == 0 {
				continue
			}
			for _, r := range results {
				inc := entity.NewIncident(
					"dictionary:"+dict.Name(),
					value.PatternID{},
					value.SeverityMedium,
					r.Fragment,
					r.StartPos,
				)
				incidents = append(incidents, *inc)
			}
		}

		incidentID := uuid.New().String()

		var resp *appshield.ScanResponse
		if engine != nil {
			resp, err = engine.Scan(c.Request.Context(), appshield.ScanRequest{
				Text:        promptText,
				ProfileSlug: tenantSlug,
			})
			if err != nil {
				log.Warn("engine scan (profile-based) failed, using dictionary-only results",
					zap.Error(err),
					zap.String("tenant_slug", tenantSlug),
				)
			}
		}

		// Mask dictionary values in each message before passing to provider
		if len(incidents) > 0 {
			dictMaskID := mask.NewShortID()
			phCounter := 0
			for mi, msg := range chatReq.Messages {
				if msg.Content == "" {
					continue
				}
				for _, dict := range tenant.Dictionaries() {
					dd := detector.NewDictionaryDetector(dict)
					results, _ := dd.Scan(c.Request.Context(), msg.Content)
					if len(results) == 0 {
						continue
					}
					sort.Slice(results, func(i, j int) bool {
						return results[i].StartPos > results[j].StartPos
					})
					for _, r := range results {
						ph := fmt.Sprintf("{{dict.%s.%d}}", dictMaskID, phCounter)
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

		duration := time.Since(start)

		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(
			attribute.String("shield.tenant", tenantSlug),
			attribute.String("shield.status", string(respStatus(resp, incidents))),
			attribute.String("shield.incident_id", incidentID),
		)

		scanStatus := string(respStatus(resp, incidents))
		metrics.ShieldScanDuration.WithLabelValues(tenantSlug, scanStatus).Observe(float64(duration.Milliseconds()))
		metrics.ShieldProfilesEvaluated.WithLabelValues(tenantSlug).Inc()

		log.Info("shield scan",
			zap.String("shield_status", scanStatus),
			zap.String("tenant_slug", tenantSlug),
			zap.String("model", chatReq.Model),
			zap.Duration("latency", duration),
			zap.String("incident_id", incidentID),
			zap.Int("dictionary_incidents", len(incidents)),
		)

		status := respStatus(resp, incidents)
		switch status {
		case value.ScanStatusBlocked:
			metrics.ShieldIncidentsBySeverity.WithLabelValues("blocked").Inc()
			c.Header("X-Shield-Status", "blocked")
			c.Header("X-Shield-Incident-ID", incidentID)
			c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
				ShieldStatus: "blocked",
				IncidentID:   incidentID,
				Error:        "request blocked by content shield",
			})

		case value.ScanStatusError:
			metrics.ShieldIncidentsBySeverity.WithLabelValues("error").Inc()
			abortWithShieldError(c, http.StatusBadGateway, "shield scan error", tenantSlug)

		case value.ScanStatusSuspicious:
			metrics.ShieldIncidentsBySeverity.WithLabelValues("suspicious").Inc()
			if cfg != nil && cfg.ActionOnSuspicious == "block" {
				c.Header("X-Shield-Status", "blocked")
				c.Header("X-Shield-Incident-ID", incidentID)
				c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
					ShieldStatus: "blocked",
					IncidentID:   incidentID,
					Error:        "request blocked by content shield",
				})
			} else {
				setShieldCleanHeaders(c, incidentID)
				c.Next()
			}

		default:
			metrics.ShieldIncidentsBySeverity.WithLabelValues("clean").Inc()
			setShieldCleanHeaders(c, incidentID)
			c.Next()
		}
	}
}

func respStatus(resp *appshield.ScanResponse, incidents []entity.Incident) value.ScanStatus {
	if resp != nil {
		return resp.Status()
	}
	if len(incidents) == 0 {
		return value.ScanStatusClean
	}
	blocked := false
	for _, inc := range incidents {
		if inc.Severity() == value.SeverityCritical {
			blocked = true
			break
		}
	}
	if blocked {
		return value.ScanStatusBlocked
	}
	return value.ScanStatusSuspicious
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

func setShieldCleanHeaders(c *gin.Context, incidentID string) {
	c.Header("X-Shield-Status", "clean")
	if incidentID != "" {
		c.Header("X-Shield-Incident-ID", incidentID)
	}
}

func abortWithShieldError(c *gin.Context, status int, message string, profileSlug string) {
	incidentID := uuid.New().String()
	c.Header("X-Shield-Status", "error")
	c.Header("X-Shield-Incident-ID", incidentID)
	c.AbortWithStatusJSON(status, shieldResponse{
		ShieldStatus: "error",
		IncidentID:   incidentID,
		Error:        message,
	})
}
