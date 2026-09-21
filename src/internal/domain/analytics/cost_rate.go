package analytics

import "fmt"

// DefaultCurrency is applied when a cost rate has no explicit currency set.
const DefaultCurrency = "USD"

// @sk-task 130-analytics-domain#T1.1: Implement CostRate value object (AC-004)
//
// CostRate represents a domain entity or configuration.
type CostRate struct {
	Model            string
	InputPricePer1K  float64
	OutputPricePer1K float64
	Currency         string
	// Source records provenance: "yaml" (seeded at startup) or "ui" (edited via admin).
	Source string
}

func NewCostRate(model string, inputPricePer1K, outputPricePer1K float64) (*CostRate, error) {
	return NewCostRateWithCurrency(model, inputPricePer1K, outputPricePer1K, DefaultCurrency)
}

func NewCostRateWithCurrency(model string, inputPricePer1K, outputPricePer1K float64, currency string) (*CostRate, error) {
	if model == "" {
		return nil, fmt.Errorf("model must not be empty")
	}
	if inputPricePer1K < 0 {
		return nil, fmt.Errorf("input price must not be negative")
	}
	if outputPricePer1K < 0 {
		return nil, fmt.Errorf("output price must not be negative")
	}
	if currency == "" {
		currency = DefaultCurrency
	}
	return &CostRate{
		Model:            model,
		InputPricePer1K:  inputPricePer1K,
		OutputPricePer1K: outputPricePer1K,
		Currency:         currency,
	}, nil
}

func (c *CostRate) Cost(inputTokens, outputTokens int64) float64 {
	inputK := float64(inputTokens) / 1000.0
	outputK := float64(outputTokens) / 1000.0
	return inputK*c.InputPricePer1K + outputK*c.OutputPricePer1K
}

// @sk-task usage-accounting-integrity#T1.4: CostRateSource records rate provenance (AC-006, AC-007)
//
// CostRateSource describes where a resolved rate came from.
type CostRateSource string

const (
	// CostRateSourceExplicit is a per-model rate configured by the operator.
	CostRateSourceExplicit CostRateSource = "explicit"
	// CostRateSourceFallback is the configured default rate for unrated models.
	CostRateSourceFallback CostRateSource = "fallback"
	// CostRateSourceMissing means no rate was available; cost is zero.
	CostRateSourceMissing CostRateSource = "missing"
)

// @sk-task 131-analytics-pipeline#T2.2: Implement CostRateRegistry (AC-007)
//
// CostRateRegistry represents a domain entity or configuration.
type CostRateRegistry struct {
	rates    map[string]*CostRate
	fallback *CostRate
}

func NewCostRateRegistry(rates []*CostRate) *CostRateRegistry {
	return NewCostRateRegistryWithFallback(rates, nil)
}

// @sk-task usage-accounting-integrity#T1.4: registry with fallback rate (AC-006)
//
// NewCostRateRegistryWithFallback builds a registry that prices models without
// an explicit entry using the given fallback rate (nil disables the fallback).
func NewCostRateRegistryWithFallback(rates []*CostRate, fallback *CostRate) *CostRateRegistry {
	m := make(map[string]*CostRate, len(rates))
	for _, r := range rates {
		m[r.Model] = r
	}
	return &CostRateRegistry{rates: m, fallback: fallback}
}

func (r *CostRateRegistry) Lookup(model string) *CostRate {
	cr, ok := r.rates[model]
	if !ok {
		return &CostRate{Model: model, InputPricePer1K: 0, OutputPricePer1K: 0, Currency: DefaultCurrency}
	}
	return cr
}

// @sk-task usage-accounting-integrity#T1.4: Resolve returns rate + provenance (AC-006, AC-007)
//
// Resolve returns the rate for a model together with where it came from:
// an explicit per-model rate, the configured fallback, or missing (zero cost).
func (r *CostRateRegistry) Resolve(model string) (*CostRate, CostRateSource) {
	if cr, ok := r.rates[model]; ok {
		return cr, CostRateSourceExplicit
	}
	if r.fallback != nil {
		return r.fallback, CostRateSourceFallback
	}
	return &CostRate{Model: model, InputPricePer1K: 0, OutputPricePer1K: 0, Currency: DefaultCurrency}, CostRateSourceMissing
}
