package dto

import "time"

// @sk-task 132-analytics-api#T1.1: Analytics DTO types (AC-001, AC-002, AC-003, AC-004, AC-005)

type AnalyticsQuery struct {
	Period  string `form:"period"`
	From    string `form:"from"`
	To      string `form:"to"`
	Model   string `form:"model"`
	Page    int    `form:"page"`
	PerPage int    `form:"per_page"`
	Format  string `form:"format"`
}

type TokenRecord struct {
	TenantID          string    `json:"tenant_id"`
	Model             string    `json:"model"`
	TotalInputTokens  int64     `json:"total_input_tokens"`
	TotalOutputTokens int64     `json:"total_output_tokens"`
	PeriodStart       time.Time `json:"period_start"`
	PeriodEnd         time.Time `json:"period_end"`
}

type TokenTotals struct {
	TotalInputTokens  int64 `json:"total_input_tokens"`
	TotalOutputTokens int64 `json:"total_output_tokens"`
}

type TokensResponse struct {
	Records []TokenRecord `json:"records"`
	Totals  TokenTotals   `json:"totals"`
}

type CostRecord struct {
	TenantID     string    `json:"tenant_id"`
	Model        string    `json:"model"`
	TotalCost    float64   `json:"total_cost"`
	RequestCount int64     `json:"request_count"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
}

type CostTotals struct {
	TotalCost    float64 `json:"total_cost"`
	RequestCount int64   `json:"request_count"`
}

type CostResponse struct {
	Records []CostRecord `json:"records"`
	Totals  CostTotals   `json:"totals"`
}

type TimeSeriesRecord struct {
	Bucket       time.Time `json:"bucket"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	Cost         float64   `json:"cost"`
	Requests     int64     `json:"requests"`
}

type TimeSeriesResponse struct {
	Series []TimeSeriesRecord `json:"series"`
	Totals struct {
		TotalTokens int64   `json:"total_tokens"`
		TotalCost   float64 `json:"total_cost"`
		Requests    int64   `json:"requests"`
	} `json:"totals"`
}

type TrafficResponse struct {
	RequestCount  int64   `json:"request_count"`
	AvgLatencyMs  *float64 `json:"avg_latency_ms"`
	P50LatencyMs  *float64 `json:"p50_latency_ms"`
	P95LatencyMs  *float64 `json:"p95_latency_ms"`
	P99LatencyMs  *float64 `json:"p99_latency_ms"`
}

type ModelBreakdown struct {
	Model      string  `json:"model"`
	Tokens     int64   `json:"tokens"`
	Cost       float64 `json:"cost"`
	Percentage float64 `json:"percentage"`
}

type TenantSummaryResponse struct {
	TenantID       string           `json:"tenant_id"`
	TotalTokens    int64            `json:"total_tokens"`
	TotalCost      float64          `json:"total_cost"`
	RequestCount   int64            `json:"request_count"`
	ModelBreakdown []ModelBreakdown `json:"model_breakdown"`
}
