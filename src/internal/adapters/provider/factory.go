package provider

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task 110-provider-adapters#T2.2: Implement NewProviderClient factory (AC-001)
// @sk-task 116-connection-pool-fixes#T2.3: Create per-provider transport with timeout (AC-002, AC-008)
// @sk-task 116-connection-pool-fixes#T3.4: Wire CircuitBreaker into provider client (AC-006, AC-007)
func NewProviderClient(pcfg *config.ProviderConfig, egressCfg *config.EgressConfig) (ports.ProviderClient, error) {
	if pcfg.APIType == "" {
		return nil, fmt.Errorf("provider %q: api_type is required", pcfg.Name)
	}

	tp := egress.NewTransport(egressCfg)
	timeout := parseTimeout(pcfg.Timeout, egressCfg.IdleTimeout)
	var cb *egress.CircuitBreaker
	if egressCfg.CircuitBreaker != nil {
		cb = egress.NewCircuitBreaker(egressCfg.CircuitBreaker)
	}
	ec := egress.NewClientWithTransport(egressCfg, tp, timeout, cb)

	switch pcfg.APIType {
	case "openai":
		return newOpenAIClient(pcfg, ec), nil
	case "anthropic":
		return newAnthropicClient(pcfg, ec), nil
	default:
		return nil, fmt.Errorf("provider %q: unsupported api_type %q", pcfg.Name, pcfg.APIType)
	}
}

// @sk-task 116-connection-pool-fixes#T2.3: Parse ProviderConfig.Timeout with fallback (AC-002)
func parseTimeout(s string, defaultTimeout time.Duration) time.Duration {
	if s == "" {
		return defaultTimeout
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Warn("invalid provider timeout, using default", "timeout", s, "default", defaultTimeout)
		return defaultTimeout
	}
	return d
}
