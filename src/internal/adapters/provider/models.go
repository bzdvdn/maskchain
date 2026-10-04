package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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
	modelsMaxAttempts    = 3
	maxModelsBody        = 4 << 20 // 4 MiB
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
// without a models API return routing.ErrModelsUnsupported. Transient transport
// errors (connection reset / unexpected EOF, common behind CDN-protected
// providers such as OpenRouter) and 429/5xx responses are retried.
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

	var lastErr error
	for attempt := 1; attempt <= modelsMaxAttempts; attempt++ {
		ids, retry, attemptErr := d.fetchModels(ctx, client, endpoint, p, parse)
		if attemptErr == nil {
			return ids, nil
		}
		lastErr = attemptErr
		if !retry || attempt == modelsMaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, lastErr
		case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
		}
	}
	return nil, lastErr
}

// fetchModels performs one discovery attempt and reports whether the failure is
// worth retrying.
func (d *ModelDiscoverer) fetchModels(ctx context.Context, client *http.Client, endpoint string, p routing.ProviderConfig, parse func([]byte) ([]string, error)) ([]string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, fmt.Errorf("provider %s: request: %w", p.Name, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "maskchain")
	applyProviderAuth(req, p)
	for k, v := range p.AdditionalHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, isRetryableNetErr(err), fmt.Errorf("provider %s: %w", p.Name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxModelsBody))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retry, fmt.Errorf("provider %s: models endpoint status %d", p.Name, resp.StatusCode)
	}
	if readErr != nil {
		return nil, true, fmt.Errorf("provider %s: read models: %w", p.Name, readErr)
	}

	ids, err := parse(body)
	if err != nil {
		return nil, false, fmt.Errorf("provider %s: parse models: %w", p.Name, err)
	}
	sort.Strings(ids)
	return ids, false, nil
}

// isRetryableNetErr reports whether a transport error is transient.
func isRetryableNetErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := err.Error()
	for _, s := range []string{"unexpected EOF", "connection reset", "broken pipe", "connection refused", "timeout"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

func modelsEndpoint(p routing.ProviderConfig) (string, func([]byte) ([]string, error), error) {
	var parse func([]byte) ([]string, error)
	switch p.APIType {
	case "openai", "proxy", "anthropic":
		parse = parseOpenAIModels
	case "ollama":
		parse = parseOllamaModels
	default:
		return "", nil, fmt.Errorf("%w: api_type %q", routing.ErrModelsUnsupported, p.APIType)
	}

	base := strings.TrimRight(p.BaseURL, "/")
	if custom := strings.TrimSpace(p.ModelsEndpoint); custom != "" {
		if strings.HasPrefix(custom, "http://") || strings.HasPrefix(custom, "https://") {
			return custom, parse, nil
		}
		if !strings.HasPrefix(custom, "/") {
			custom = "/" + custom
		}
		return joinBasePath(base, custom), parse, nil
	}

	// Default discovery path, joined with the same /v1 de-dup rule the chat
	// proxies use, so a base URL with or without /v1 resolves correctly.
	defaultPath := "/v1/models"
	if p.APIType == "ollama" {
		defaultPath = "/api/tags"
	}
	return joinBasePath(base, defaultPath), parse, nil
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
