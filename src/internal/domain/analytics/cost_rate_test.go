package analytics

import "testing"

// @sk-test usage-accounting-integrity#T4.1: Resolve returns explicit/fallback/missing provenance (AC-006, AC-007)
func TestCostRateRegistryResolve(t *testing.T) {
	explicit := &CostRate{Model: "gpt-4o", InputPricePer1K: 1, OutputPricePer1K: 2, Currency: DefaultCurrency}
	fallback := &CostRate{Model: "default", InputPricePer1K: 0.5, OutputPricePer1K: 1.5, Currency: DefaultCurrency}
	reg := NewCostRateRegistryWithFallback([]*CostRate{explicit}, fallback)

	got, source := reg.Resolve("gpt-4o")
	if source != CostRateSourceExplicit || got != explicit {
		t.Errorf("explicit: got %+v source=%s", got, source)
	}

	got, source = reg.Resolve("unlisted-model")
	if source != CostRateSourceFallback || got != fallback {
		t.Errorf("fallback: got %+v source=%s", got, source)
	}
}

// @sk-test usage-accounting-integrity#T4.1: no fallback resolves to missing with zero cost (AC-007)
func TestCostRateRegistryResolveMissing(t *testing.T) {
	reg := NewCostRateRegistry([]*CostRate{
		{Model: "gpt-4o", InputPricePer1K: 1, OutputPricePer1K: 2, Currency: DefaultCurrency},
	})

	got, source := reg.Resolve("unlisted-model")
	if source != CostRateSourceMissing {
		t.Errorf("expected missing source, got %s", source)
	}
	if got.InputPricePer1K != 0 || got.OutputPricePer1K != 0 {
		t.Errorf("expected zero rate, got %+v", got)
	}
	if cost := got.Cost(1000, 1000); cost != 0 {
		t.Errorf("expected zero cost, got %f", cost)
	}
}

// @sk-test usage-accounting-integrity#T4.1: fallback rate computes cost for unlisted models (AC-006)
func TestCostRateRegistryFallbackCost(t *testing.T) {
	fallback := &CostRate{Model: "default", InputPricePer1K: 1, OutputPricePer1K: 3, Currency: DefaultCurrency}
	reg := NewCostRateRegistryWithFallback(nil, fallback)

	got, source := reg.Resolve("brand-new-model")
	if source != CostRateSourceFallback {
		t.Fatalf("expected fallback source, got %s", source)
	}
	if cost := got.Cost(1000, 1000); cost != 4 {
		t.Errorf("expected cost 4, got %f", cost)
	}
}
