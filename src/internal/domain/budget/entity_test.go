package budget

import (
	"testing"
	"time"
)

func TestBudgetKeyPrefix(t *testing.T) {
	if KeyPrefixBudget != "budget:" {
		t.Errorf("KeyPrefixBudget = %q, want %q", KeyPrefixBudget, "budget:")
	}
}

func TestNewBudgetValidation(t *testing.T) {
	if _, err := NewBudget("", "t1", ScopeTenant, PeriodMonthly); err == nil {
		t.Error("expected error for empty id")
	}
	if _, err := NewBudget("b1", "", ScopeTenant, PeriodMonthly); err == nil {
		t.Error("expected error for empty tenant")
	}
	if _, err := NewBudget("b1", "t1", Scope("bad"), PeriodMonthly); err == nil {
		t.Error("expected error for invalid scope")
	}
	if _, err := NewBudget("b1", "t1", ScopeTenant, PeriodType("bad")); err == nil {
		t.Error("expected error for invalid period type")
	}
	b, err := NewBudget("b1", "t1", ScopeTenant, PeriodMonthly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Currency != DefaultCurrency {
		t.Errorf("Currency = %q, want %q", b.Currency, DefaultCurrency)
	}
	if !b.Enabled {
		t.Error("expected Enabled to be true by default")
	}
}

func TestBudgetPeriodKey(t *testing.T) {
	now := time.Date(2026, 8, 11, 15, 30, 0, 0, time.UTC)

	monthly, _ := NewBudget("b1", "t1", ScopeTenant, PeriodMonthly)
	if got := monthly.PeriodKey(now); got != "2026-08" {
		t.Errorf("monthly PeriodKey = %q, want 2026-08", got)
	}

	daily, _ := NewBudget("b2", "t1", ScopeTenant, PeriodDaily)
	if got := daily.PeriodKey(now); got != "2026-08-11" {
		t.Errorf("daily PeriodKey = %q, want 2026-08-11", got)
	}

	custom, _ := NewBudget("b3", "t1", ScopeTenant, PeriodCustom)
	custom.CustomDays = 7
	if got := custom.PeriodKey(now); got == "" {
		t.Error("expected non-empty custom period key")
	}
}

func TestBudgetCounterKey(t *testing.T) {
	now := time.Date(2026, 8, 11, 15, 30, 0, 0, time.UTC)

	tenant, _ := NewBudget("b1", "t1", ScopeTenant, PeriodMonthly)
	want := "budget:tenant:t1:2026-08"
	if got := tenant.CounterKey(now); got != want {
		t.Errorf("tenant CounterKey = %q, want %q", got, want)
	}

	key, _ := NewBudget("b2", "t1", ScopeKey, PeriodMonthly)
	key.VirtualKeyID = "k1"
	want = "budget:key:k1:2026-08"
	if got := key.CounterKey(now); got != want {
		t.Errorf("key CounterKey = %q, want %q", got, want)
	}

	model, _ := NewBudget("b3", "t1", ScopeModel, PeriodMonthly)
	model.Model = "gpt-4"
	want = "budget:model:t1:gpt-4:2026-08"
	if got := model.CounterKey(now); got != want {
		t.Errorf("model CounterKey = %q, want %q", got, want)
	}
}

func TestBudgetHardAndSoftExceeded(t *testing.T) {
	hard := 100.0
	soft := 50.0
	b, _ := NewBudget("b1", "t1", ScopeTenant, PeriodMonthly)
	b.HardLimit = &hard
	b.SoftLimit = &soft

	if b.HardExceeded(99) {
		t.Error("HardExceeded(99) = true, want false")
	}
	if !b.HardExceeded(100) {
		t.Error("HardExceeded(100) = false, want true")
	}
	if !b.HardExceeded(150) {
		t.Error("HardExceeded(150) = false, want true")
	}
	if b.SoftExceeded(49) {
		t.Error("SoftExceeded(49) = true, want false")
	}
	if !b.SoftExceeded(50) {
		t.Error("SoftExceeded(50) = false, want true")
	}
	if b.HardExceeded(0) {
		t.Error("HardExceeded(0) with nil limit = true, want false")
	}
}

func TestBudgetCrossedThresholds(t *testing.T) {
	hard := 100.0
	b, _ := NewBudget("b1", "t1", ScopeTenant, PeriodMonthly)
	b.HardLimit = &hard
	b.NotifyAt = []float64{50, 80, 100}

	crossed := b.CrossedThresholds(0, 55)
	if len(crossed) != 1 || crossed[0] != 50 {
		t.Errorf("CrossedThresholds(0,55) = %v, want [50]", crossed)
	}

	crossed = b.CrossedThresholds(50, 90)
	if len(crossed) != 1 || crossed[0] != 80 {
		t.Errorf("CrossedThresholds(50,90) = %v, want [80]", crossed)
	}

	crossed = b.CrossedThresholds(90, 100)
	if len(crossed) != 1 || crossed[0] != 100 {
		t.Errorf("CrossedThresholds(90,100) = %v, want [100]", crossed)
	}

	if crossed := b.CrossedThresholds(10, 20); len(crossed) != 0 {
		t.Errorf("CrossedThresholds(10,20) = %v, want []", crossed)
	}
}

func TestBudgetPeriodTTL(t *testing.T) {
	now := time.Date(2026, 8, 11, 15, 30, 0, 0, time.UTC)

	monthly, _ := NewBudget("b1", "t1", ScopeTenant, PeriodMonthly)
	ttl := monthly.PeriodTTL(now)
	if ttl <= 0 || ttl > 30*24*time.Hour {
		t.Errorf("monthly PeriodTTL = %v, want in (0, 31d]", ttl)
	}

	daily, _ := NewBudget("b2", "t1", ScopeTenant, PeriodDaily)
	ttl = daily.PeriodTTL(now)
	if ttl <= 0 || ttl > 24*time.Hour {
		t.Errorf("daily PeriodTTL = %v, want in (0, 24h]", ttl)
	}
}
