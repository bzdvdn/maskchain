package analytics

import "fmt"

// @sk-task 130-analytics-domain#T1.1: Implement CostRate value object (AC-004)
//
// CostRate represents a domain entity or configuration.
type CostRate struct {
	Model            string
	InputPricePer1K  float64
	OutputPricePer1K float64
}

func NewCostRate(model string, inputPricePer1K, outputPricePer1K float64) (*CostRate, error) {
	if model == "" {
		return nil, fmt.Errorf("model must not be empty")
	}
	if inputPricePer1K < 0 {
		return nil, fmt.Errorf("input price must not be negative")
	}
	if outputPricePer1K < 0 {
		return nil, fmt.Errorf("output price must not be negative")
	}
	return &CostRate{
		Model:            model,
		InputPricePer1K:  inputPricePer1K,
		OutputPricePer1K: outputPricePer1K,
	}, nil
}

func (c *CostRate) Cost(inputTokens, outputTokens int64) float64 {
	inputK := float64(inputTokens) / 1000.0
	outputK := float64(outputTokens) / 1000.0
	return inputK*c.InputPricePer1K + outputK*c.OutputPricePer1K
}

// @sk-task 131-analytics-pipeline#T2.2: Implement CostRateRegistry (AC-007)
//
// CostRateRegistry represents a domain entity or configuration.
type CostRateRegistry struct {
	rates map[string]*CostRate
}

func NewCostRateRegistry(rates []*CostRate) *CostRateRegistry {
	m := make(map[string]*CostRate, len(rates))
	for _, r := range rates {
		m[r.Model] = r
	}
	return &CostRateRegistry{rates: m}
}

func (r *CostRateRegistry) Lookup(model string) *CostRate {
	cr, ok := r.rates[model]
	if !ok {
		return &CostRate{Model: model, InputPricePer1K: 0, OutputPricePer1K: 0}
	}
	return cr
}
