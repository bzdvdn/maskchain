package egress

import (
	"sync"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 116-connection-pool-fixes#T3.3: Implement CircuitBreaker with configurable max failures and cooldown (AC-006, AC-007)
type CircuitBreaker struct {
	cfg      *config.CircuitBreakerConfig
	mu       sync.Mutex
	failures int64
	deadline int64
}

func NewCircuitBreaker(cfg *config.CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{cfg: cfg}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.deadline == 0 {
		return true
	}
	return time.Now().UnixNano() >= cb.deadline
}

func (cb *CircuitBreaker) Fail() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	if cb.failures >= int64(cb.cfg.MaxFailures) {
		cb.deadline = time.Now().Add(cb.cfg.Cooldown).UnixNano()
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.deadline = 0
}
