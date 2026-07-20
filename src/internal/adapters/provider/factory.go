package provider

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task provider-egress-proxy#T4.1: Pass pcfg.ProxyURL to egress.NewTransport
// @sk-task 110-provider-adapters#T2.2: Implement NewProviderClient factory (AC-001)
// @sk-task 116-connection-pool-fixes#T2.3: Create per-provider transport with timeout (AC-002, AC-008)
// @sk-task 116-connection-pool-fixes#T3.4: Wire CircuitBreaker into provider client (AC-006, AC-007)
//
// NewProviderClient creates a new ProviderClient.
func NewProviderClient(pcfg *config.ProviderConfig, egressCfg *config.EgressConfig) (ports.ProviderClient, error) {
	if pcfg.APIType == "" {
		return nil, fmt.Errorf("provider %q: api_type is required", pcfg.Name)
	}

	tp, err := egress.NewTransport(egressCfg, pcfg.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("provider %q: failed to create transport: %w", pcfg.Name, err)
	}
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
	// @sk-task ollama-provider#T2.2: Register ollama in factory (AC-001)
	case "ollama":
		return newOllamaClient(pcfg, ec), nil
	// @sk-task provider-adapters-expansion#T5.1: Wire proxy, gemini, and bedrock adapters (AC-001, AC-004, AC-007)
	case "proxy":
		return newProxyClient(pcfg, ec), nil
	case "gemini":
		return newGeminiClient(pcfg, ec), nil
	case "bedrock":
		return newBedrockClient(pcfg)
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
