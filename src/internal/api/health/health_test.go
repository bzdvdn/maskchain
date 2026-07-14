package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

var errMockFail = errors.New("mock probe failure")

type mockProbe struct {
	name   string
	status string
	err    error
}

func (m *mockProbe) Name() string {
	return m.name
}

func (m *mockProbe) Check(ctx context.Context) Result {
	return NewResult(m.status, 0, m.err)
}

// @sk-test 114-real-health-probes#T4.1: TestAggregationAllOk (AC-002, AC-007)
func TestAggregationAllOk(t *testing.T) {
	svc := NewService([]string{"database"})
	svc.Register(&mockProbe{name: "database", status: "ok"})
	svc.Register(&mockProbe{name: "valkey", status: "ok"})

	res := svc.CheckAll(context.Background())
	if res.Status != "ok" {
		t.Errorf("expected ok, got %s", res.Status)
	}
	if len(res.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(res.Checks))
	}
	for name, cs := range res.Checks {
		if cs.Status != "ok" {
			t.Errorf("check %s: expected ok, got %s", name, cs.Status)
		}
	}
}

// @sk-test 114-real-health-probes#T4.1: TestAggregationDegraded (AC-003, AC-007)
func TestAggregationDegraded(t *testing.T) {
	svc := NewService([]string{"database"})
	svc.Register(&mockProbe{name: "database", status: "ok"})
	svc.Register(&mockProbe{name: "valkey", status: "down", err: errMockFail})

	res := svc.CheckAll(context.Background())
	if res.Status != "degraded" {
		t.Errorf("expected degraded, got %s", res.Status)
	}
	if res.Checks["valkey"].Status != "down" {
		t.Errorf("expected valkey down, got %s", res.Checks["valkey"].Status)
	}
}

// @sk-test 114-real-health-probes#T4.1: TestAggregationDown (AC-004, AC-007)
func TestAggregationDown(t *testing.T) {
	svc := NewService([]string{"database"})
	svc.Register(&mockProbe{name: "database", status: "down", err: errMockFail})
	svc.Register(&mockProbe{name: "valkey", status: "ok"})

	res := svc.CheckAll(context.Background())
	if res.Status != "down" {
		t.Errorf("expected down, got %s", res.Status)
	}
}

// @sk-test 114-real-health-probes#T4.1: TestCriticalDepsConfig (AC-006)
func TestCriticalDepsConfig(t *testing.T) {
	// valkey not in critical_deps -> degraded, not down
	svc := NewService([]string{"database"})
	svc.Register(&mockProbe{name: "database", status: "ok"})
	svc.Register(&mockProbe{name: "valkey", status: "down", err: errMockFail})

	res := svc.CheckAll(context.Background())
	if res.Status != "degraded" {
		t.Errorf("expected degraded (valkey non-critical), got %s", res.Status)
	}

	// both in critical_deps -> down
	svc2 := NewService([]string{"database", "valkey"})
	svc2.Register(&mockProbe{name: "database", status: "ok"})
	svc2.Register(&mockProbe{name: "valkey", status: "down", err: errMockFail})

	res2 := svc2.CheckAll(context.Background())
	if res2.Status != "down" {
		t.Errorf("expected down (valkey critical), got %s", res2.Status)
	}
}

// @sk-test 114-real-health-probes#T4.1: TestNilDependencyProbe (AC-002)
func TestNilDependencyProbe(t *testing.T) {
	svc := NewService([]string{"database"})
	svc.Register(NewPGProbe(nil))
	svc.Register(NewValkeyProbe(nil))

	res := svc.CheckAll(context.Background())
	if res.Status != "ok" {
		t.Errorf("expected ok for nil deps, got %s", res.Status)
	}
}

// @sk-test 114-real-health-probes#T4.1: TestLivenessHandler (AC-001, AC-007)
func TestLivenessHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(nil)
	h := NewHandler(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	h.LivenessHandler()(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"status":"ok"}` {
		t.Errorf("expected {\"status\":\"ok\"}, got %s", w.Body.String())
	}
}

// @sk-test 114-real-health-probes#T4.1: TestStartupHandler (AC-005, AC-007)
func TestStartupHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(nil)
	h := NewHandler(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/live", nil)

	h.StartupHandler()(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"status":"ok"}` {
		t.Errorf("expected {\"status\":\"ok\"}, got %s", w.Body.String())
	}
}

// @sk-test 114-real-health-probes#T4.1: TestReadinessHandlerOk (AC-002, AC-007)
func TestReadinessHandlerOk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(nil)
	svc.Register(&mockProbe{name: "database", status: "ok"})
	h := NewHandler(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	h.ReadinessHandler()(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// @sk-test 114-real-health-probes#T4.1: TestReadinessHandlerDown (AC-004)
func TestReadinessHandlerDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService([]string{"database"})
	svc.Register(&mockProbe{name: "database", status: "down", err: errMockFail})
	h := NewHandler(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	h.ReadinessHandler()(ctx)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

// @sk-test 114-real-health-probes#T4.1: TestLatencyMsInResponse (AC-007)
func TestLatencyMsInResponse(t *testing.T) {
	svc := NewService(nil)
	svc.Register(&mockProbe{name: "database", status: "ok"})

	res := svc.CheckAll(context.Background())
	cs, ok := res.Checks["database"]
	if !ok {
		t.Fatal("expected database check")
	}
	if cs.LatencyMs < 0 {
		t.Errorf("expected non-negative latency_ms, got %d", cs.LatencyMs)
	}
}
