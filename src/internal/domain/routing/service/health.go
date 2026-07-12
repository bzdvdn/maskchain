package service

import (
	"context"
	"net/http"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-task 70-routing-engine#T2.4: Implement HealthChecker (AC-006)
type HealthChecker struct {
	registry *ProviderRegistry
	client   *http.Client
}

func NewHealthChecker(registry *ProviderRegistry, client *http.Client) *HealthChecker {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HealthChecker{registry: registry, client: client}
}

func (c *HealthChecker) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.checkAll()
	for {
		select {
		case <-ticker.C:
			c.checkAll()
		case <-ctx.Done():
			return
		}
	}
}

func (c *HealthChecker) checkAll() {
	for _, p := range c.registry.List() {
		if p.HealthEndpoint == "" {
			p.SetHealthStatus(routing.HealthHealthy)
			continue
		}
		resp, err := c.client.Get(p.BaseURL + p.HealthEndpoint)
		if err != nil || resp.StatusCode != http.StatusOK {
			p.SetHealthStatus(routing.HealthUnhealthy)
			continue
		}
		_ = resp.Body.Close()
		p.SetHealthStatus(routing.HealthHealthy)
	}
}
