package provider

import (
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task 110-provider-adapters#T2.2: Implement NewProviderClient factory (AC-001)
func NewProviderClient(pcfg *config.ProviderConfig, egressCfg *config.EgressConfig) (ports.ProviderClient, error) {
	if pcfg.APIType == "" {
		return nil, fmt.Errorf("provider %q: api_type is required", pcfg.Name)
	}

	ec := egress.NewClient(egressCfg)

	switch pcfg.APIType {
	case "openai":
		return newOpenAIClient(pcfg, ec), nil
	case "anthropic":
		return newAnthropicClient(pcfg, ec), nil
	default:
		return nil, fmt.Errorf("provider %q: unsupported api_type %q", pcfg.Name, pcfg.APIType)
	}
}
