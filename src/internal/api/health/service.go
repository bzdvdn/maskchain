package health

import (
	"context"
	"sync"
	"time"
)

type AggregatedResult struct {
	Status string                `json:"status"`
	Checks map[string]CheckState `json:"checks,omitempty"`
}

type CheckState struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// @sk-task 114-real-health-probes#T1.1: HealthService with Register/CheckAll (AC-007)
type HealthService struct {
	mu          sync.RWMutex
	probes      []Probe
	criticalDep map[string]bool
}

func NewService(criticalDeps []string) *HealthService {
	s := &HealthService{
		criticalDep: make(map[string]bool, len(criticalDeps)),
	}
	for _, d := range criticalDeps {
		s.criticalDep[d] = true
	}
	return s
}

func (s *HealthService) Register(p Probe) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probes = append(s.probes, p)
}

func (s *HealthService) CheckAll(ctx context.Context) *AggregatedResult {
	s.mu.RLock()
	probes := make([]Probe, len(s.probes))
	copy(probes, s.probes)
	s.mu.RUnlock()

	res := &AggregatedResult{
		Status: "ok",
		Checks: make(map[string]CheckState, len(probes)),
	}

	for _, p := range probes {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		checkStart := time.Now()
		r := p.Check(probeCtx)
		elapsed := time.Since(checkStart)
		cancel()

		cs := CheckState{
			Status:    r.Status,
			LatencyMs: elapsed.Milliseconds(),
			Error:     r.Error,
		}
		res.Checks[p.Name()] = cs
	}

	criticalDown := false
	nonCriticalDown := false
	for _, p := range probes {
		cs := res.Checks[p.Name()]
		if cs.Status == "ok" {
			continue
		}
		if s.criticalDep[p.Name()] {
			criticalDown = true
		} else {
			nonCriticalDown = true
		}
	}

	switch {
	case criticalDown:
		res.Status = "down"
	case nonCriticalDown:
		res.Status = "degraded"
	default:
		res.Status = "ok"
	}

	return res
}

func (s *HealthService) IsReady(ctx context.Context) *AggregatedResult {
	return s.CheckAll(ctx)
}
