package health

import (
	"context"
	"time"
)

// @sk-task 114-real-health-probes#T1.1: Probe interface and Result type (AC-007)
type Probe interface {
	Name() string
	Check(ctx context.Context) Result
}

// @sk-task 114-real-health-probes#T1.1: Probe interface and Result type (AC-007)
type Result struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

func NewResult(status string, latency time.Duration, err error) Result {
	r := Result{
		Status:    status,
		LatencyMs: latency.Milliseconds(),
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}
