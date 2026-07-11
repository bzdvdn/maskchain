package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "maskchain"

// @sk-task 61-observability#T1.3: RegisterMetrics registers all Prometheus metrics (AC-002, AC-003, AC-004)
func RegisterMetrics(reg *prometheus.Registry) {
	reg.MustRegister(HttpRequestsTotal)
	reg.MustRegister(HttpRequestDuration)
	reg.MustRegister(ShieldScanDuration)
	reg.MustRegister(ShieldIncidentsBySeverity)
	reg.MustRegister(ShieldProfilesEvaluated)
}

// @sk-task 61-observability#T2.1: Middleware returns gin middleware that records HTTP request metrics (AC-003)
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		status := strconv.Itoa(c.Writer.Status())
		HttpRequestDuration.WithLabelValues(c.Request.Method, c.Request.URL.Path, status).Observe(float64(duration.Milliseconds()))
		HttpRequestsTotal.WithLabelValues(c.Request.Method, c.Request.URL.Path, status).Inc()
	}
}

// @sk-task 61-observability#T1.3: Handler returns gin handler for /metrics (AC-002)
func Handler(reg *prometheus.Registry) gin.HandlerFunc {
	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

var (
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_ms",
			Help:      "HTTP request duration in milliseconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status_code"},
	)

	ShieldScanDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "shield_scan_duration_ms",
			Help:      "Shield scan duration in milliseconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"profile", "status"},
	)

	ShieldIncidentsBySeverity = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "shield_incidents_by_severity",
			Help:      "Shield incidents by severity status",
		},
		[]string{"status"},
	)

	ShieldProfilesEvaluated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "shield_profiles_evaluated",
			Help:      "Number of profiles evaluated by shield",
		},
		[]string{"profile"},
	)
)
