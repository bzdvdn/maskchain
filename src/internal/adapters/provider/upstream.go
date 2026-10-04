package provider

import (
	"fmt"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task provider-path-fidelity#T1.2: Resolve upstream path from the request (AC-001, AC-002, AC-003, AC-005, AC-009)
//
// openAIShapedEndpoints lists the request paths an OpenAI-shaped provider
// (openai, proxy, ollama) can serve. The gateway only mounts these exact
// routes, so matching is exact rather than suffix-based.
var openAIShapedEndpoints = map[string]bool{
	"/v1/chat/completions": true,
	"/v1/completions":      true,
	"/v1/embeddings":       true,
}

// ResolveUpstreamURL builds the upstream URL for a passthrough provider from the
// incoming request path. It validates that the provider supports the requested
// endpoint shape, de-duplicates a /v1 segment between baseURL and the path, and
// appends the original query string verbatim.
//
// It returns ports.ErrUnsupportedEndpoint (wrapped) when the provider cannot
// serve the requested shape, so callers can distinguish it from transport
// errors.
func ResolveUpstreamURL(apiType, baseURL string, req *ports.ProviderRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("resolve upstream url: nil request")
	}
	path, err := sanitizePath(req.URL)
	if err != nil {
		return "", err
	}
	upstream, ok := upstreamPathFor(apiType, path)
	if !ok {
		return "", fmt.Errorf("%w: api_type %q cannot serve %s", ports.ErrUnsupportedEndpoint, apiType, path)
	}
	joined := joinBasePath(baseURL, upstream)
	if req.RawQuery != "" {
		joined += "?" + req.RawQuery
	}
	return joined, nil
}

// sanitizePath rejects client-controlled inputs that could redirect the request
// away from the configured provider host or smuggle a query/fragment.
func sanitizePath(raw string) (string, error) {
	switch {
	case raw == "":
		return "", fmt.Errorf("invalid request path: empty")
	case !strings.HasPrefix(raw, "/"):
		return "", fmt.Errorf("invalid request path %q: must start with /", raw)
	case strings.HasPrefix(raw, "//"):
		return "", fmt.Errorf("invalid request path %q: scheme-relative paths are not allowed", raw)
	case strings.Contains(raw, "://"):
		return "", fmt.Errorf("invalid request path %q: absolute URLs are not allowed", raw)
	case strings.Contains(raw, ".."):
		return "", fmt.Errorf("invalid request path %q: path traversal is not allowed", raw)
	case strings.ContainsAny(raw, "?#"):
		return "", fmt.Errorf("invalid request path %q: query and fragment must be sent via RawQuery", raw)
	}
	return raw, nil
}

// upstreamPathFor maps the incoming request path to the provider-native upstream
// path. Anthropic keeps its historical chat alias, which has always been
// forwarded to the messages endpoint.
func upstreamPathFor(apiType, requestPath string) (string, bool) {
	switch apiType {
	case "openai", "proxy", "ollama":
		if openAIShapedEndpoints[requestPath] {
			return requestPath, true
		}
		return "", false
	case "anthropic":
		if requestPath == "/v1/messages" || requestPath == "/v1/chat/completions" {
			return "/v1/messages", true
		}
		return "", false
	default:
		return "", false
	}
}

// joinBasePath joins a provider base URL and an upstream path without ever
// duplicating a /v1 segment (e.g. base .../v1 + path /v1/embeddings).
func joinBasePath(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/v1") && (path == "/v1" || strings.HasPrefix(path, "/v1/")) {
		path = strings.TrimPrefix(path, "/v1")
	}
	return base + path
}
