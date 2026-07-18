package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type mockUsageStore struct {
	records      []analytics.UsageRecord
	aggregations []analytics.Aggregation
	err          error
}

func (m *mockUsageStore) Record(ctx context.Context, usage analytics.TokenUsage) error {
	return m.err
}

func (m *mockUsageStore) RecordBatch(ctx context.Context, usages []analytics.TokenUsage) error {
	return m.err
}

func (m *mockUsageStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return 0, m.err
}

func (m *mockUsageStore) QueryByTenant(ctx context.Context, tenantID value.TenantID, from, to time.Time) ([]analytics.UsageRecord, error) {
	return m.records, m.err
}

func (m *mockUsageStore) QueryByModel(ctx context.Context, model string, from, to time.Time) ([]analytics.UsageRecord, error) {
	return m.records, m.err
}

func (m *mockUsageStore) QueryAll(ctx context.Context, from, to time.Time) ([]analytics.UsageRecord, error) {
	return m.records, m.err
}

func (m *mockUsageStore) AggregateByDay(ctx context.Context, tenantID value.TenantID, from, to time.Time) ([]analytics.Aggregation, error) {
	return m.aggregations, m.err
}

func (m *mockUsageStore) QueryTimeSeries(ctx context.Context, from, to time.Time) ([]analytics.TimeSeriesPoint, error) {
	return nil, m.err
}

func setupTest(t *testing.T, store *mockUsageStore) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewAnalyticsHandler(store)
	engine := gin.New()
	group := engine.Group("/api/v1/analytics")
	group.GET("/tokens", h.HandleTokens)
	group.GET("/cost", h.HandleCost)
	group.GET("/traffic", h.HandleTraffic)
	return engine
}

func setTenant(c *gin.Context, slug string) {
	s, _ := value.NewTenantSlug(slug)
	t := entity.NewTenant(s, "", "", nil)
	c.Set("tenant", t)
}

func now() time.Time {
	return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
}

// @sk-test 132-analytics-api#T3.1: TestAnalyticsHandler_Tokens (AC-001)
func TestAnalyticsHandler_Tokens(t *testing.T) {
	store := &mockUsageStore{
		records: []analytics.UsageRecord{
			{TenantID: "tenant-a", Model: "gpt-4", TotalInputTokens: 100, TotalOutputTokens: 50, PeriodStart: now().AddDate(0, 0, -1), PeriodEnd: now()},
			{TenantID: "tenant-a", Model: "gpt-3.5", TotalInputTokens: 200, TotalOutputTokens: 100, PeriodStart: now().AddDate(0, 0, -1), PeriodEnd: now()},
		},
	}
	engine := setupTest(t, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?period=week", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data struct {
			Records []struct {
				TenantID          string `json:"tenant_id"`
				Model             string `json:"model"`
				TotalInputTokens  int64  `json:"total_input_tokens"`
				TotalOutputTokens int64  `json:"total_output_tokens"`
			} `json:"records"`
			Totals struct {
				TotalInputTokens  int64 `json:"total_input_tokens"`
				TotalOutputTokens int64 `json:"total_output_tokens"`
			} `json:"totals"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Records) != 2 {
		t.Errorf("expected 2 records, got %d", len(resp.Data.Records))
	}
	if resp.Data.Totals.TotalInputTokens != 300 {
		t.Errorf("expected 300 total input tokens, got %d", resp.Data.Totals.TotalInputTokens)
	}
	if resp.Data.Totals.TotalOutputTokens != 150 {
		t.Errorf("expected 150 total output tokens, got %d", resp.Data.Totals.TotalOutputTokens)
	}
}

// @sk-test 132-analytics-api#T3.1: TestAnalyticsHandler_Cost (AC-002)
func TestAnalyticsHandler_Cost(t *testing.T) {
	store := &mockUsageStore{
		records: []analytics.UsageRecord{
			{TenantID: "tenant-a", Model: "gpt-4", TotalCost: 5.50, RequestCount: 10, PeriodStart: now().AddDate(0, 0, -1), PeriodEnd: now()},
			{TenantID: "tenant-a", Model: "gpt-3.5", TotalCost: 2.25, RequestCount: 20, PeriodStart: now().AddDate(0, 0, -1), PeriodEnd: now()},
		},
	}
	engine := setupTest(t, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/analytics/cost?period=month", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data struct {
			Records []struct {
				TenantID     string  `json:"tenant_id"`
				Model        string  `json:"model"`
				TotalCost    float64 `json:"total_cost"`
				RequestCount int64   `json:"request_count"`
			} `json:"records"`
			Totals struct {
				TotalCost    float64 `json:"total_cost"`
				RequestCount int64   `json:"request_count"`
			} `json:"totals"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Records) != 2 {
		t.Errorf("expected 2 records, got %d", len(resp.Data.Records))
	}
	if resp.Data.Totals.TotalCost != 7.75 {
		t.Errorf("expected 7.75 total cost, got %f", resp.Data.Totals.TotalCost)
	}
	if resp.Data.Totals.RequestCount != 30 {
		t.Errorf("expected 30 total requests, got %d", resp.Data.Totals.RequestCount)
	}
}

// @sk-test 132-analytics-api#T3.1: TestAnalyticsHandler_Traffic (AC-003)
func TestAnalyticsHandler_Traffic(t *testing.T) {
	store := &mockUsageStore{
		records: []analytics.UsageRecord{
			{RequestCount: 15},
			{RequestCount: 25},
		},
	}
	engine := setupTest(t, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/analytics/traffic?period=day", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data struct {
			RequestCount int64   `json:"request_count"`
			AvgLatencyMs *float64 `json:"avg_latency_ms"`
			P50LatencyMs *float64 `json:"p50_latency_ms"`
			P95LatencyMs *float64 `json:"p95_latency_ms"`
			P99LatencyMs *float64 `json:"p99_latency_ms"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.RequestCount != 40 {
		t.Errorf("expected 40 total requests, got %d", resp.Data.RequestCount)
	}
	if resp.Data.AvgLatencyMs != nil {
		t.Errorf("expected nil avg_latency_ms, got %v", resp.Data.AvgLatencyMs)
	}
}

// @sk-test 132-analytics-api#T3.1: TestAnalyticsHandler_CSVExport (AC-005)
func TestAnalyticsHandler_CSVExport(t *testing.T) {
	store := &mockUsageStore{
		records: []analytics.UsageRecord{
			{TenantID: "tenant-a", Model: "gpt-4", TotalInputTokens: 100, TotalOutputTokens: 50, PeriodStart: now(), PeriodEnd: now()},
		},
	}
	engine := setupTest(t, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?period=week&format=csv", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "text/csv" {
		t.Errorf("expected text/csv, got %s", w.Header().Get("Content-Type"))
	}
	if w.Body.Len() == 0 {
		t.Errorf("expected non-empty CSV body")
	}
}

// @sk-test 132-analytics-api#T3.1: TestAnalyticsHandler_InvalidPeriod (AC-001)
func TestAnalyticsHandler_InvalidPeriod(t *testing.T) {
	store := &mockUsageStore{}
	engine := setupTest(t, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?period=invalid", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (falls back to week), got %d", w.Code)
	}
}

// @sk-test 132-analytics-api#T3.1: TestAnalyticsHandler_Pagination (AC-005)
func TestAnalyticsHandler_Pagination(t *testing.T) {
	records := make([]analytics.UsageRecord, 25)
	for i := range records {
		records[i] = analytics.UsageRecord{
			TenantID:          "tenant-a",
			Model:             "gpt-4",
			TotalInputTokens:  int64(i + 1),
			TotalOutputTokens: int64(i + 1),
			PeriodStart:       now(),
			PeriodEnd:         now(),
		}
	}
	store := &mockUsageStore{records: records}
	engine := setupTest(t, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?page=2&per_page=10", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data       json.RawMessage `json:"data"`
		Pagination struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Pagination.Page != 2 {
		t.Errorf("expected page=2, got %d", resp.Pagination.Page)
	}
	if resp.Pagination.Total != 25 {
		t.Errorf("expected total=25, got %d", resp.Pagination.Total)
	}
}
