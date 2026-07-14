package egress

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 116-connection-pool-fixes#T3.3: Implement CircuitBreaker with configurable max failures and cooldown (AC-006, AC-007)
type CircuitBreaker struct {
	cfg      *config.CircuitBreakerConfig
	mu       sync.Mutex
	failures atomic.Int64
	deadline atomic.Int64
}

func NewCircuitBreaker(cfg *config.CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{cfg: cfg}
}

func (cb *CircuitBreaker) Allow() bool {
	deadline := cb.deadline.Load()
	if deadline == 0 {
		return true
	}
	if time.Now().UnixNano() < deadline {
		return false
	}
	return true
}

func (cb *CircuitBreaker) Fail() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	n := cb.failures.Add(1)
	if n >= int64(cb.cfg.MaxFailures) {
		cb.deadline.Store(time.Now().Add(cb.cfg.Cooldown).UnixNano())
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures.Store(0)
	cb.deadline.Store(0)
}
