package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// fakeRegistryRepo is an in-memory routing.RegistryRepository for handler tests.
type fakeRegistryRepo struct {
	providers []routingDomain.ProviderConfig
}

func (f *fakeRegistryRepo) ListProviders(_ context.Context) ([]routingDomain.ProviderConfig, error) {
	out := make([]routingDomain.ProviderConfig, len(f.providers))
	copy(out, f.providers)
	return out, nil
}

func (f *fakeRegistryRepo) ListRules(_ context.Context) ([]routingDomain.RuleConfig, error) {
	return nil, nil
}

func (f *fakeRegistryRepo) UpsertProvider(_ context.Context, p routingDomain.ProviderConfig) error {
	for i, existing := range f.providers {
		if existing.Name == p.Name {
			f.providers[i] = p
			return nil
		}
	}
	f.providers = append(f.providers, p)
	return nil
}

func (f *fakeRegistryRepo) DeleteProvider(_ context.Context, name string) error {
	for i, p := range f.providers {
		if p.Name == name {
			f.providers = append(f.providers[:i], f.providers[i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeRegistryRepo) UpsertRoute(_ context.Context, _, _ string, _ []string) error {
	return nil
}

func (f *fakeRegistryRepo) DeleteRoute(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeRegistryRepo) SeedFromYAML(_ context.Context, providers []routingDomain.ProviderConfig, _ []routingDomain.RuleConfig) (bool, error) {
	f.providers = providers
	return true, nil
}

// dtoProviderResponse mirrors the secret fields of dto.ProviderResponse so the
// masking test can decode the JSON body without a package import cycle.
type dtoProviderResponse struct {
	Name               string   `json:"name"`
	APIKeys            []string `json:"api_keys"`
	AWSAccessKeyID     string   `json:"aws_access_key_id"`
	AWSSecretAccessKey string   `json:"aws_secret_access_key"`
}

func hasMaskMarker(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i] == '*' && s[i+1] == '*' && s[i+2] == '*' {
			return true
		}
	}
	return false
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestRoutingHandlerListMasksSecrets (AC-003)
func TestRoutingHandlerListMasksSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRegistryRepo{providers: []routingDomain.ProviderConfig{
		{
			Name:               "openai",
			APIType:            "openai",
			BaseURL:            "https://api.openai.com/v1",
			APIKeys:            []string{"sk-longsecret-123456789"},
			AWSAccessKeyID:     "AKIAACCESSKEYID0000",
			AWSSecretAccessKey: "supersecretvalue00000",
		},
	}}
	h := NewRoutingHandler(repo, nil, nil)
	router := gin.New()
	router.GET("/api/v1/routing/providers", h.ListProviders)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routing/providers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []dtoProviderResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(resp.Data))
	}
	p := resp.Data[0]
	if len(p.APIKeys) != 1 || p.APIKeys[0] == "sk-longsecret-123456789" {
		t.Errorf("expected api_keys masked, got %v", p.APIKeys)
	}
	if !hasMaskMarker(p.APIKeys[0]) {
		t.Errorf("expected api_keys to contain mask marker, got %q", p.APIKeys[0])
	}
	if p.AWSAccessKeyID == "AKIAACCESSKEYID0000" {
		t.Error("expected aws_access_key_id masked")
	}
	if p.AWSSecretAccessKey == "supersecretvalue00000" {
		t.Error("expected aws_secret_access_key masked")
	}
}
