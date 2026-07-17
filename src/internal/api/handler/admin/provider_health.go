package admin

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// @sk-task admin-ui-design#T4.3: ProviderHealthChecker pings providers (AC-006)
type ProviderHealth struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	LastCheck int64  `json:"last_check"`
	Error     string `json:"error,omitempty"`
}

type ProviderTarget struct {
	Name           string
	BaseURL        string
	HealthEndpoint string
}

type ProviderHealthChecker struct {
	mu      sync.RWMutex
	results map[string]*ProviderHealth
	client  *http.Client
}

func NewProviderHealthChecker(timeout time.Duration) *ProviderHealthChecker {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &ProviderHealthChecker{
		results: make(map[string]*ProviderHealth),
		client:  &http.Client{Timeout: timeout},
	}
}

func healthURL(baseURL, endpoint string) string {
	if endpoint == "" {
		endpoint = "/"
	}
	return baseURL + endpoint
}

// Check pings the given provider and stores the result.
func (c *ProviderHealthChecker) Check(ctx context.Context, target ProviderTarget) *ProviderHealth {
	url := healthURL(target.BaseURL, target.HealthEndpoint)
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		h := &ProviderHealth{Status: "down", LastCheck: time.Now().Unix(), Error: err.Error()}
		c.store(target.Name, h)
		return h
	}

	resp, err := c.client.Do(req)
	elapsed := time.Since(start)
	h := &ProviderHealth{LastCheck: time.Now().Unix()}

	if err != nil {
		h.Status = "down"
		h.Error = err.Error()
	} else {
		resp.Body.Close()
		h.LatencyMs = elapsed.Milliseconds()
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			h.Status = "up"
		} else {
			h.Status = "down"
		}
	}

	c.store(target.Name, h)
	return h
}

func (c *ProviderHealthChecker) store(name string, h *ProviderHealth) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[name] = h
}

func (c *ProviderHealthChecker) GetResult(name string) *ProviderHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.results[name]
}

// RefreshAll checks all providers.
func (c *ProviderHealthChecker) RefreshAll(ctx context.Context, targets []ProviderTarget) {
	for _, t := range targets {
		c.Check(ctx, t)
	}
}

// StartBackgroundRefresh runs health checks every interval until ctx is done.
func (c *ProviderHealthChecker) StartBackgroundRefresh(ctx context.Context, interval time.Duration, targets []ProviderTarget) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	c.RefreshAll(ctx, targets)
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.RefreshAll(ctx, targets)
			}
		}
	}()
}
