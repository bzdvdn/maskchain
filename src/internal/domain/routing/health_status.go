package routing

import "sync/atomic"

// @sk-task 70-routing-engine#T1.1: Create HealthStatus enum (AC-006)
type HealthStatus int32

const (
	HealthUnknown   HealthStatus = 0
	HealthHealthy   HealthStatus = 1
	HealthUnhealthy HealthStatus = 2
)

func (s HealthStatus) String() string {
	switch s {
	case HealthHealthy:
		return "healthy"
	case HealthUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// @sk-task 70-routing-engine#T1.1: Create Provider entity with health status (AC-001, AC-006)
type Provider struct {
	Name           string
	BaseURL        string
	HealthEndpoint string
	Timeout        string
	Priority       int
	healthStatus   atomic.Int32
}

func NewProvider(name, baseURL, healthEndpoint, timeout string, priority int) *Provider {
	return &Provider{
		Name:           name,
		BaseURL:        baseURL,
		HealthEndpoint: healthEndpoint,
		Timeout:        timeout,
		Priority:       priority,
	}
}

func (p *Provider) HealthStatus() HealthStatus {
	return HealthStatus(p.healthStatus.Load())
}

func (p *Provider) SetHealthStatus(s HealthStatus) {
	p.healthStatus.Store(int32(s))
}
