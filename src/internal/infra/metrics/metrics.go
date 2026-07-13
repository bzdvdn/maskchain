package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
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
	reg.MustRegister(RateLimitExceededTotal)
	reg.MustRegister(RateLimitRemaining)
	reg.MustRegister(DictionaryCacheHitsTotal)
	reg.MustRegister(DictionaryCacheMissesTotal)
	reg.MustRegister(DictionaryCacheStaleTotal)
	reg.MustRegister(DictionaryCacheInvalidationsTotal)
}

// @sk-task 90-production-hardening#T3.2: Register PG pool metrics collector (<AC-003>)
func RegisterPGPoolCollector(reg *prometheus.Registry, pool *pgxpool.Pool) {
	reg.MustRegister(NewPGPoolCollector(pool))
}

// @sk-task 61-observability#T2.1: Middleware returns gin middleware that records HTTP request metrics (AC-003)
// @sk-task 80-tenant-isolation#T3.3: Add tenant label to HTTP metrics (AC-009)
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		status := strconv.Itoa(c.Writer.Status())
		tenant := "unknown"
		if tid, ok := c.Get("tenant_slug"); ok {
			if s, ok := tid.(string); ok {
				tenant = s
			}
		}
		HttpRequestDuration.WithLabelValues(c.Request.Method, c.Request.URL.Path, status, tenant).Observe(float64(duration.Milliseconds()))
		HttpRequestsTotal.WithLabelValues(c.Request.Method, c.Request.URL.Path, status, tenant).Inc()
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
	// @sk-task 80-tenant-isolation#T3.3: Add tenant label to HTTP metrics (AC-009)
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code", "tenant"},
	)

	// @sk-task 80-tenant-isolation#T3.3: Add tenant label to HTTP metrics (AC-009)
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_ms",
			Help:      "HTTP request duration in milliseconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status_code", "tenant"},
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

	// @sk-task rate-limiting-budgets#T3.3: Add rate limit Prometheus metrics (AC-007)
	RateLimitExceededTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rate_limited_total",
			Help:      "Total number of rate-limited requests",
		},
		[]string{"tenant", "reason"},
	)

	// @sk-task rate-limiting-budgets#T3.3: Add rate limit remaining gauge (AC-007)
	RateLimitRemaining = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "rate_limit_remaining",
			Help:      "Remaining requests in current window",
		},
		[]string{"tenant"},
	)

	// @sk-task tenant-profile-sync#T4.1: Register DictionaryCache Prometheus counter vectors (AC-006)
	DictionaryCacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dictionary_cache_hits_total",
			Help:      "Total number of dictionary cache hits",
		},
		[]string{"operation", "level"},
	)

	DictionaryCacheMissesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dictionary_cache_misses_total",
			Help:      "Total number of dictionary cache misses",
		},
		[]string{"operation", "level"},
	)

	DictionaryCacheStaleTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dictionary_cache_stale_total",
			Help:      "Total number of stale cache reads (Valkey degraded)",
		},
		[]string{"operation"},
	)

	DictionaryCacheInvalidationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dictionary_cache_invalidations_total",
			Help:      "Total number of cache invalidations",
		},
		[]string{"operation"},
	)
)
