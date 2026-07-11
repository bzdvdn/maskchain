package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newTestRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	RegisterMetrics(reg)
	return reg
}

// @sk-test 61-observability#T4.1: TestMetricsPrefix verifies all metrics have maskchain_ prefix (AC-002)
func TestMetricsPrefix(t *testing.T) {
	HttpRequestsTotal.WithLabelValues("GET", "/test", "200").Inc()
	HttpRequestDuration.WithLabelValues("GET", "/test", "200").Observe(10)
	ShieldScanDuration.WithLabelValues("test-profile", "clean").Observe(10)
	ShieldIncidentsBySeverity.WithLabelValues("blocked").Inc()
	ShieldProfilesEvaluated.WithLabelValues("test-profile").Inc()

	reg := newTestRegistry()
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "maskchain_") {
		t.Error("expected metrics with maskchain_ prefix")
	}
	if !strings.Contains(body, "maskchain_http_requests_total") {
		t.Error("expected maskchain_http_requests_total")
	}
	if !strings.Contains(body, "maskchain_http_request_duration_ms") {
		t.Error("expected maskchain_http_request_duration_ms")
	}
	if !strings.Contains(body, "maskchain_shield_scan_duration_ms") {
		t.Error("expected maskchain_shield_scan_duration_ms")
	}
	if !strings.Contains(body, "maskchain_shield_incidents_by_severity") {
		t.Error("expected maskchain_shield_incidents_by_severity")
	}
	if !strings.Contains(body, "maskchain_shield_profiles_evaluated") {
		t.Error("expected maskchain_shield_profiles_evaluated")
	}
}

// @sk-test 61-observability#T4.1: TestHTTPRequestDuration verifies histogram records after request (AC-003)
func TestHTTPRequestDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	RegisterMetrics(reg)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Middleware())
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(w2, req2)

	body := w2.Body.String()
	if !strings.Contains(body, "maskchain_http_request_duration_ms_count") {
		t.Error("expected duration count metric")
	}
	if !strings.Contains(body, "maskchain_http_requests_total") {
		t.Error("expected total requests metric")
	}
}

// @sk-test 61-observability#T4.1: TestShieldMetrics verifies shield metrics record correctly (AC-004)
func TestShieldMetrics(t *testing.T) {
	ShieldScanDuration.WithLabelValues("test-profile", "clean").Observe(10)
	ShieldIncidentsBySeverity.WithLabelValues("blocked").Inc()
	ShieldProfilesEvaluated.WithLabelValues("test-profile").Inc()

	reg := prometheus.NewRegistry()
	RegisterMetrics(reg)

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "shield_scan_duration_ms") {
		t.Error("expected shield_scan_duration_ms in metrics")
	}
	if !strings.Contains(body, "shield_incidents_by_severity") {
		t.Error("expected shield_incidents_by_severity in metrics")
	}
	if !strings.Contains(body, "shield_profiles_evaluated") {
		t.Error("expected shield_profiles_evaluated in metrics")
	}
}
