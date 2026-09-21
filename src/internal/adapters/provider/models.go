package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

const (
	modelsRequestTimeout = 15 * time.Second
	maxModelsBody        = 2 << 20 // 2 MiB
)

// @sk-task routing-ia#T5.1: provider model discovery (AC-011)
//
// ModelDiscoverer lists the models a provider exposes via its own models API,
// honoring the provider's auth scheme and egress proxy.
type ModelDiscoverer struct {
	egressCfg *config.EgressConfig
}

func NewModelDiscoverer(egressCfg *config.EgressConfig) *ModelDiscoverer {
	return &ModelDiscoverer{egressCfg: egressCfg}
}

// Discover returns the sorted model ids the provider exposes. Provider types
// without a models API return routing.ErrModelsUnsupported.
func (d *ModelDiscoverer) Discover(ctx context.Context, p routing.ProviderConfig) ([]string, error) {
	endpoint, parse, err := modelsEndpoint(p)
	if err != nil {
		return nil, err
	}

	egressCfg := d.egressCfg
	if egressCfg == nil {
		egressCfg = &config.EgressConfig{}
	}
	tp, err := egress.NewTransport(egressCfg, p.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("provider %s: transport: %w", p.Name, err)
	}
	client := &http.Client{Transport: tp, Timeout: modelsRequestTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("provider %s: request: %w", p.Name, err)
	}
	applyProviderAuth(req, p)
	for k, v := range p.AdditionalHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider %s: %w", p.Name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxModelsBody))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider %s: models endpoint status %d", p.Name, resp.StatusCode)
	}

	ids, err := parse(body)
	if err != nil {
		return nil, fmt.Errorf("provider %s: parse models: %w", p.Name, err)
	}
	sort.Strings(ids)
	return ids, nil
}

func modelsEndpoint(p routing.ProviderConfig) (string, func([]byte) ([]string, error), error) {
	base := strings.TrimRight(p.BaseURL, "/")
	switch p.APIType {
	case "openai", "proxy":
		return base + "/models", parseOpenAIModels, nil
	case "anthropic":
		return base + "/v1/models", parseOpenAIModels, nil
	case "ollama":
		return base + "/api/tags", parseOllamaModels, nil
	default:
		return "", nil, fmt.Errorf("%w: api_type %q", routing.ErrModelsUnsupported, p.APIType)
	}
}

func parseOpenAIModels(body []byte) ([]string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

func parseOllamaModels(body []byte) ([]string, error) {
	var out struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		if name != "" {
			ids = append(ids, name)
		}
	}
	return ids, nil
}

// applyProviderAuth mirrors the chat adapters' header rules.
func applyProviderAuth(req *http.Request, p routing.ProviderConfig) {
	scheme := p.AuthScheme
	if scheme == "" {
		scheme = "bearer"
	}
	header := p.AuthHeader
	if header == "" {
		header = "Authorization"
	}
	if len(p.APIKeys) > 0 && p.APIKeys[0] != "" {
		key, value := buildAuthHeader(scheme, header, p.AuthPrefix, p.APIKeys[0])
		req.Header.Set(key, value)
	}
	if p.APIType == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}
