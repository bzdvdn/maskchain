package provider

import (
	"errors"
	"strings"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task provider-path-fidelity#T1.3: Table tests for upstream URL composition (AC-005, AC-008, AC-009)
func TestResolveUpstreamURL(t *testing.T) {
	tests := []struct {
		name            string
		apiType         string
		baseURL         string
		path            string
		query           string
		want            string
		wantUnsupported bool
		wantErr         bool
	}{
		{
			name:    "openai chat on root base",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "/v1/chat/completions",
			want:    "https://api.openai.com/v1/chat/completions",
		},
		{
			name:    "openai embeddings",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "/v1/embeddings",
			want:    "https://api.openai.com/v1/embeddings",
		},
		{
			name:    "openai completions",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "/v1/completions",
			want:    "https://api.openai.com/v1/completions",
		},
		{
			name:    "proxy base ending in v1 is not duplicated",
			apiType: "proxy",
			baseURL: "https://api.groq.com/openai/v1",
			path:    "/v1/chat/completions",
			want:    "https://api.groq.com/openai/v1/chat/completions",
		},
		{
			name:    "proxy base without v1 gains one",
			apiType: "proxy",
			baseURL: "https://api.groq.com/openai",
			path:    "/v1/embeddings",
			want:    "https://api.groq.com/openai/v1/embeddings",
		},
		{
			name:    "base with trailing slash",
			apiType: "ollama",
			baseURL: "http://localhost:11434/",
			path:    "/v1/chat/completions",
			want:    "http://localhost:11434/v1/chat/completions",
		},
		{
			name:    "query string is preserved",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "/v1/chat/completions",
			query:   "foo=bar&baz=1",
			want:    "https://api.openai.com/v1/chat/completions?foo=bar&baz=1",
		},
		{
			name:    "anthropic messages native",
			apiType: "anthropic",
			baseURL: "https://api.anthropic.com",
			path:    "/v1/messages",
			want:    "https://api.anthropic.com/v1/messages",
		},
		{
			name:    "anthropic chat alias maps to messages",
			apiType: "anthropic",
			baseURL: "https://api.anthropic.com",
			path:    "/v1/chat/completions",
			want:    "https://api.anthropic.com/v1/messages",
		},
		{
			name:            "openai cannot serve messages",
			apiType:         "openai",
			baseURL:         "https://api.openai.com",
			path:            "/v1/messages",
			wantUnsupported: true,
		},
		{
			name:            "anthropic cannot serve embeddings",
			apiType:         "anthropic",
			baseURL:         "https://api.anthropic.com",
			path:            "/v1/embeddings",
			wantUnsupported: true,
		},
		{
			name:            "unknown api type is unsupported",
			apiType:         "gemini",
			baseURL:         "https://example.com",
			path:            "/v1/chat/completions",
			wantUnsupported: true,
		},
		{
			name:    "empty path",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "",
			wantErr: true,
		},
		{
			name:    "relative path without leading slash",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "v1/chat/completions",
			wantErr: true,
		},
		{
			name:    "scheme-relative path",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "//evil.example/v1/chat/completions",
			wantErr: true,
		},
		{
			name:    "absolute url",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "https://evil.example/v1/chat/completions",
			wantErr: true,
		},
		{
			name:    "path traversal",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "/v1/../../secret",
			wantErr: true,
		},
		{
			name:    "query smuggled in path",
			apiType: "openai",
			baseURL: "https://api.openai.com",
			path:    "/v1/chat/completions?x=1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveUpstreamURL(tt.apiType, tt.baseURL, &ports.ProviderRequest{URL: tt.path, RawQuery: tt.query})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got url %q", got)
				}
				if errors.Is(err, ports.ErrUnsupportedEndpoint) {
					t.Fatalf("expected validation error, got unsupported endpoint: %v", err)
				}
				return
			}
			if tt.wantUnsupported {
				if !errors.Is(err, ports.ErrUnsupportedEndpoint) {
					t.Fatalf("expected ErrUnsupportedEndpoint, got err=%v url=%q", err, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("url = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, "/v1/v1") {
				t.Fatalf("url %q contains duplicated /v1", got)
			}
		})
	}
}

// @sk-task provider-path-fidelity#T1.3: nil request must not panic (AC-009)
func TestResolveUpstreamURL_NilRequest(t *testing.T) {
	if _, err := ResolveUpstreamURL("openai", "https://api.openai.com", nil); err == nil {
		t.Fatal("expected error for nil request")
	}
}
