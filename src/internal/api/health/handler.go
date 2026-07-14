package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *HealthService
}

func NewHandler(svc *HealthService) *Handler {
	return &Handler{svc: svc}
}

// @sk-task 114-real-health-probes#T2.1: Liveness handler always returns ok (AC-001)
func (h *Handler) LivenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// @sk-task 114-real-health-probes#T2.1: Readiness handler with dynamic probe aggregation (AC-002, AC-003, AC-004)
func (h *Handler) ReadinessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		res := h.svc.CheckAll(c.Request.Context())

		statusCode := http.StatusOK
		if res.Status == "down" {
			statusCode = http.StatusServiceUnavailable
		}
		c.JSON(statusCode, res)
	}
}

// @sk-task 114-real-health-probes#T2.1: Startup handler returns ok after init (AC-005)
func (h *Handler) StartupHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
