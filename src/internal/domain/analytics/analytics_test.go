package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 130-analytics-domain#T3.1: TestTokenUsage (AC-001)
func TestTokenUsage(t *testing.T) {
	t.Run("creates with valid fields", func(t *testing.T) {
		tenantID, _ := value.NewTenantID("tenant-1")
		now := time.Now()

		u, err := NewTokenUsage(tenantID, "gpt-4", 100, 50, 0.015, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.TenantID.String() != "tenant-1" {
			t.Errorf("TenantID = %q, want %q", u.TenantID.String(), "tenant-1")
		}
		if u.Model != "gpt-4" {
			t.Errorf("Model = %q, want %q", u.Model, "gpt-4")
		}
		if u.InputTokens != 100 {
			t.Errorf("InputTokens = %d, want %d", u.InputTokens, 100)
		}
		if u.OutputTokens != 50 {
			t.Errorf("OutputTokens = %d, want %d", u.OutputTokens, 50)
		}
		if u.Cost != 0.015 {
			t.Errorf("Cost = %f, want %f", u.Cost, 0.015)
		}
		if !u.Timestamp.Equal(now) {
			t.Errorf("Timestamp = %v, want %v", u.Timestamp, now)
		}
	})

	t.Run("rejects negative input tokens", func(t *testing.T) {
		tenantID, _ := value.NewTenantID("t1")
		_, err := NewTokenUsage(tenantID, "gpt-4", -1, 0, 0, time.Now())
		if err == nil {
			t.Fatal("expected error for negative input tokens")
		}
	})

	t.Run("rejects negative output tokens", func(t *testing.T) {
		tenantID, _ := value.NewTenantID("t1")
		_, err := NewTokenUsage(tenantID, "gpt-4", 0, -1, 0, time.Now())
		if err == nil {
			t.Fatal("expected error for negative output tokens")
		}
	})

	t.Run("rejects negative cost", func(t *testing.T) {
		tenantID, _ := value.NewTenantID("t1")
		_, err := NewTokenUsage(tenantID, "gpt-4", 0, 0, -0.01, time.Now())
		if err == nil {
			t.Fatal("expected error for negative cost")
		}
	})

	t.Run("rejects empty tenantID", func(t *testing.T) {
		emptyID, _ := value.NewTenantID("")
		_, err := NewTokenUsage(emptyID, "gpt-4", 0, 0, 0, time.Now())
		if err == nil {
			t.Fatal("expected error for empty tenantID")
		}
	})

	t.Run("rejects empty model", func(t *testing.T) {
		tenantID, _ := value.NewTenantID("t1")
		_, err := NewTokenUsage(tenantID, "", 0, 0, 0, time.Now())
		if err == nil {
			t.Fatal("expected error for empty model")
		}
	})

	t.Run("accepts zero tokens (health-check)", func(t *testing.T) {
		tenantID, _ := value.NewTenantID("t1")
		u, err := NewTokenUsage(tenantID, "gpt-4", 0, 0, 0, time.Now())
		if err != nil {
			t.Fatalf("unexpected error for zero tokens: %v", err)
		}
		if u.InputTokens != 0 || u.OutputTokens != 0 {
			t.Error("expected zero tokens")
		}
	})
}

// @sk-test 130-analytics-domain#T3.1: TestCostRate (AC-004)
func TestCostRate(t *testing.T) {
	t.Run("computes cost correctly", func(t *testing.T) {
		cr, err := NewCostRate("gpt-4", 0.01, 0.03)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cost := cr.Cost(500, 200)
		expected := 0.5*0.01 + 0.2*0.03 // 0.011
		if cost != expected {
			t.Errorf("Cost = %f, want %f", cost, expected)
		}
	})

	t.Run("zero price is valid", func(t *testing.T) {
		cr, err := NewCostRate("free-model", 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cost := cr.Cost(1000, 1000)
		if cost != 0 {
			t.Errorf("Cost = %f, want 0", cost)
		}
	})

	t.Run("zero tokens cost zero", func(t *testing.T) {
		cr, _ := NewCostRate("gpt-4", 0.01, 0.03)
		cost := cr.Cost(0, 0)
		if cost != 0 {
			t.Errorf("Cost = %f, want 0", cost)
		}
	})

	t.Run("rejects negative input price", func(t *testing.T) {
		_, err := NewCostRate("m", -0.01, 0.03)
		if err == nil {
			t.Fatal("expected error for negative input price")
		}
	})

	t.Run("rejects negative output price", func(t *testing.T) {
		_, err := NewCostRate("m", 0.01, -0.03)
		if err == nil {
			t.Fatal("expected error for negative output price")
		}
	})

	t.Run("rejects empty model", func(t *testing.T) {
		_, err := NewCostRate("", 0.01, 0.03)
		if err == nil {
			t.Fatal("expected error for empty model")
		}
	})
}

// @sk-test 130-analytics-domain#T3.1: TestAggregation (AC-003)
func TestAggregation(t *testing.T) {
	t.Run("creates with valid fields", func(t *testing.T) {
		date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
		a := NewAggregation("tenant-1", "gpt-4", date, 1500, 0.05, 10, 50*time.Millisecond)

		if a.TenantID != "tenant-1" {
			t.Errorf("TenantID = %q, want %q", a.TenantID, "tenant-1")
		}
		if a.Model != "gpt-4" {
			t.Errorf("Model = %q, want %q", a.Model, "gpt-4")
		}
		if !a.Date.Equal(date) {
			t.Errorf("Date = %v, want %v", a.Date, date)
		}
		if a.TotalTokens != 1500 {
			t.Errorf("TotalTokens = %d, want %d", a.TotalTokens, 1500)
		}
		if a.TotalCost != 0.05 {
			t.Errorf("TotalCost = %f, want %f", a.TotalCost, 0.05)
		}
		if a.RequestCount != 10 {
			t.Errorf("RequestCount = %d, want %d", a.RequestCount, 10)
		}
		if a.AvgLatency != 50*time.Millisecond {
			t.Errorf("AvgLatency = %v, want %v", a.AvgLatency, 50*time.Millisecond)
		}
	})
}

// @sk-test 130-analytics-domain#T3.1: TestUsageRecord (AC-005)
func TestUsageRecord(t *testing.T) {
	t.Run("creates with aggregated values", func(t *testing.T) {
		start := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)

		r := NewUsageRecord("tenant-1", "gpt-4", start, end, 1000, 500, 0.15, 10)

		if r.TenantID != "tenant-1" {
			t.Errorf("TenantID = %q, want %q", r.TenantID, "tenant-1")
		}
		if r.Model != "gpt-4" {
			t.Errorf("Model = %q, want %q", r.Model, "gpt-4")
		}
		if !r.PeriodStart.Equal(start) {
			t.Errorf("PeriodStart = %v, want %v", r.PeriodStart, start)
		}
		if !r.PeriodEnd.Equal(end) {
			t.Errorf("PeriodEnd = %v, want %v", r.PeriodEnd, end)
		}
		if r.TotalInputTokens != 1000 {
			t.Errorf("TotalInputTokens = %d, want %d", r.TotalInputTokens, 1000)
		}
		if r.TotalOutputTokens != 500 {
			t.Errorf("TotalOutputTokens = %d, want %d", r.TotalOutputTokens, 500)
		}
		if r.TotalCost != 0.15 {
			t.Errorf("TotalCost = %f, want %f", r.TotalCost, 0.15)
		}
		if r.RequestCount != 10 {
			t.Errorf("RequestCount = %d, want %d", r.RequestCount, 10)
		}
	})
}

// @sk-test 130-analytics-domain#T3.1: TestUsageStoreInterface (AC-002)
func TestUsageStoreInterface(t *testing.T) {
	var _ UsageStore = (*mockUsageStore)(nil)
}

type mockUsageStore struct{}

func (m *mockUsageStore) Record(_ context.Context, _ TokenUsage) error        { return nil }
func (m *mockUsageStore) RecordBatch(_ context.Context, _ []TokenUsage) error { return nil }
func (m *mockUsageStore) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (m *mockUsageStore) QueryByTenant(_ context.Context, _ value.TenantID, _, _ time.Time) ([]UsageRecord, error) {
	return nil, nil
}
func (m *mockUsageStore) QueryByModel(_ context.Context, _ string, _, _ time.Time) ([]UsageRecord, error) {
	return nil, nil
}
func (m *mockUsageStore) QueryAll(_ context.Context, _, _ time.Time) ([]UsageRecord, error) {
	return nil, nil
}
func (m *mockUsageStore) AggregateByDay(_ context.Context, _ value.TenantID, _, _ time.Time) ([]Aggregation, error) {
	return nil, nil
}
func (m *mockUsageStore) QueryTimeSeries(_ context.Context, _, _ time.Time) ([]TimeSeriesPoint, error) {
	return nil, nil
}

func floatEqual(a, b float64) bool {
	const epsilon = 1e-9
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// @sk-test 131-analytics-pipeline#T4.5: TestCostRateRegistry (AC-007)
func TestCostRateRegistry(t *testing.T) {
	t.Run("lookup known model returns correct rate", func(t *testing.T) {
		rates := []*CostRate{
			{Model: "gpt-4", InputPricePer1K: 0.01, OutputPricePer1K: 0.03},
			{Model: "gpt-3.5-turbo", InputPricePer1K: 0.001, OutputPricePer1K: 0.002},
		}
		reg := NewCostRateRegistry(rates)
		cr := reg.Lookup("gpt-4")
		if cr.Model != "gpt-4" {
			t.Errorf("Model = %q, want %q", cr.Model, "gpt-4")
		}
		if cr.InputPricePer1K != 0.01 {
			t.Errorf("InputPricePer1K = %f, want %f", cr.InputPricePer1K, 0.01)
		}
		if cr.OutputPricePer1K != 0.03 {
			t.Errorf("OutputPricePer1K = %f, want %f", cr.OutputPricePer1K, 0.03)
		}
		cost := cr.Cost(1000, 500)
		expected := 1.0*0.01 + 0.5*0.03
		if cost != expected {
			t.Errorf("Cost = %f, want %f", cost, expected)
		}
	})

	t.Run("lookup unknown model returns zero cost rate", func(t *testing.T) {
		rates := []*CostRate{
			{Model: "gpt-4", InputPricePer1K: 0.01, OutputPricePer1K: 0.03},
		}
		reg := NewCostRateRegistry(rates)
		cr := reg.Lookup("unknown-model")
		if cr.Model != "unknown-model" {
			t.Errorf("Model = %q, want %q", cr.Model, "unknown-model")
		}
		if cr.InputPricePer1K != 0 {
			t.Errorf("InputPricePer1K = %f, want 0", cr.InputPricePer1K)
		}
		if cr.OutputPricePer1K != 0 {
			t.Errorf("OutputPricePer1K = %f, want 0", cr.OutputPricePer1K)
		}
		cost := cr.Cost(1000, 500)
		if cost != 0 {
			t.Errorf("Cost = %f, want 0", cost)
		}
	})

	t.Run("empty registry returns zero cost for any model", func(t *testing.T) {
		reg := NewCostRateRegistry(nil)
		cr := reg.Lookup("any-model")
		if cr.InputPricePer1K != 0 || cr.OutputPricePer1K != 0 {
			t.Error("expected zero cost rate from empty registry")
		}
	})

	t.Run("multiple models all return correct rates", func(t *testing.T) {
		rates := []*CostRate{
			{Model: "gpt-4", InputPricePer1K: 0.01, OutputPricePer1K: 0.03},
			{Model: "gpt-3.5-turbo", InputPricePer1K: 0.001, OutputPricePer1K: 0.002},
			{Model: "claude-3", InputPricePer1K: 0.015, OutputPricePer1K: 0.075},
		}
		reg := NewCostRateRegistry(rates)
		cost := reg.Lookup("claude-3").Cost(1, 1)
		expected := 0.015/1000 + 0.075/1000
		if !floatEqual(cost, expected) {
			t.Errorf("unexpected cost for claude-3: %f, want %f", cost, expected)
		}
	})
}
