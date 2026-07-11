package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	appshield "github.com/bzdvdn/maskchain/src/internal/app/usecase/shield"
	domshield "github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
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
func ShieldMiddleware(engine Scanner, profileRepo domshield.ProfileRepository, cfg *config.ShieldConfig, log *zap.Logger) gin.HandlerFunc {
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

		profileSlugStr := c.GetHeader("X-Shield-Profile-Slug")
		if profileSlugStr == "" {
			abortWithShieldError(c, http.StatusBadRequest, "missing X-Shield-Profile-Slug header", "")
			return
		}

		profileSlug, err := value.NewProfileSlug(profileSlugStr)
		if err != nil {
			abortWithShieldError(c, http.StatusBadRequest, fmt.Sprintf("invalid profile slug: %s", profileSlugStr), "")
			return
		}

		tenantID := resolveTenantID(c)

		profile, err := profileRepo.FindBySlug(c.Request.Context(), tenantID, profileSlug)
		if err != nil || profile == nil {
			abortWithShieldError(c, http.StatusNotFound, fmt.Sprintf("profile %s not found", profileSlug.String()), profileSlug.String())
			return
		}

		if !profile.Enabled() {
			abortWithShieldError(c, http.StatusNotFound, fmt.Sprintf("profile %s is disabled", profileSlug.String()), profileSlug.String())
			return
		}

		var chatReq chatRequest
		if err := json.Unmarshal(body, &chatReq); err != nil {
			abortWithShieldError(c, http.StatusBadRequest, "invalid JSON body", profileSlug.String())
			return
		}

		promptText := extractPromptText(chatReq.Messages)
		if promptText == "" {
			setShieldCleanHeaders(c, "")
			c.Next()
			return
		}

		incidentID := uuid.New().String()

		resp, err := engine.Scan(c.Request.Context(), appshield.ScanRequest{
			Text:        promptText,
			ProfileSlug: profileSlug.String(),
		})
		if err != nil {
			log.Error("shield scan failed", zap.Error(err), zap.String("profile_slug", profileSlug.String()))
			abortWithShieldError(c, http.StatusBadGateway, "shield scan error", profileSlug.String())
			return
		}

		duration := time.Since(start)

		log.Info("shield scan",
			zap.String("shield_status", string(resp.Status())),
			zap.String("profile_slug", profileSlug.String()),
			zap.String("model", chatReq.Model),
			zap.Duration("latency", duration),
			zap.String("incident_id", incidentID),
		)

		switch resp.Status() {
		case value.ScanStatusBlocked:
			c.Header("X-Shield-Status", "blocked")
			c.Header("X-Shield-Incident-ID", incidentID)
			c.AbortWithStatusJSON(http.StatusForbidden, shieldResponse{
				ShieldStatus: "blocked",
				IncidentID:   incidentID,
				Error:        "request blocked by content shield",
			})

		case value.ScanStatusError:
			abortWithShieldError(c, http.StatusBadGateway, "shield scan error", profileSlug.String())

		case value.ScanStatusSuspicious:
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
			setShieldCleanHeaders(c, incidentID)
			c.Next()
		}
	}
}

func resolveTenantID(c *gin.Context) value.TenantID {
	tid := c.GetHeader("X-Tenant-ID")
	if tid == "" {
		tid = "default"
	}
	id, err := value.NewTenantID(tid)
	if err != nil {
		id, _ = value.NewTenantID("default")
	}
	return id
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
