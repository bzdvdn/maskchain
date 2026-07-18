package egress

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// @sk-test provider-egress-proxy#T5.1: proxyFuncFromURL returns fixed proxy for http URL
func TestProxyFromURL(t *testing.T) {
	pf, err := proxyFuncFromURL("http://proxy:3128")
	if err != nil {
		t.Fatalf("proxyFuncFromURL: %s", err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	proxyURL, err := pf(req)
	if err != nil {
		t.Fatalf("proxy func: %s", err)
	}
	if proxyURL == nil {
		t.Fatal("expected proxy URL, got nil")
	}
	if proxyURL.String() != "http://proxy:3128" {
		t.Fatalf("expected http://proxy:3128, got %s", proxyURL.String())
	}
}

// @sk-test provider-egress-proxy#T5.1: proxyFuncFromURL returns proxy for https URL too
func TestProxyFromURLHTTPS(t *testing.T) {
	pf, err := proxyFuncFromURL("https://proxy:3128")
	if err != nil {
		t.Fatalf("proxyFuncFromURL: %s", err)
	}
	req := httptest.NewRequest(http.MethodGet, "https://api.openai.com/", nil)
	proxyURL, err := pf(req)
	if err != nil {
		t.Fatalf("proxy func: %s", err)
	}
	if proxyURL == nil {
		t.Fatal("expected proxy URL, got nil")
	}
	if proxyURL.String() != "https://proxy:3128" {
		t.Fatalf("expected https://proxy:3128, got %s", proxyURL.String())
	}
}

// @sk-test provider-egress-proxy#T5.1: empty proxyURL falls back to env var
func TestProxyFromEmptyURLFallback(t *testing.T) {
	os.Setenv("HTTP_PROXY", "http://env-proxy:8080")
	defer os.Unsetenv("HTTP_PROXY")

	pf, err := proxyFuncFromURL("")
	if err != nil {
		t.Fatalf("proxyFuncFromURL: %s", err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	proxyURL, err := pf(req)
	if err != nil {
		t.Fatalf("proxy func: %s", err)
	}
	if proxyURL == nil {
		t.Fatal("expected proxy URL from env, got nil")
	}
	expected := "http://env-proxy:8080"
	if proxyURL.String() != expected {
		t.Fatalf("expected %s, got %s", expected, proxyURL.String())
	}
}

// @sk-test provider-egress-proxy#T5.1: empty proxyURL + no env var = direct (no proxy)
func TestProxyFromEmptyURLNoEnv(t *testing.T) {
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")

	pf, err := proxyFuncFromURL("")
	if err != nil {
		t.Fatalf("proxyFuncFromURL: %s", err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	proxyURL, err := pf(req)
	if err != nil {
		t.Fatalf("proxy func: %s", err)
	}
	if proxyURL != nil {
		t.Fatalf("expected nil proxy (direct), got %s", proxyURL.String())
	}
}

// @sk-test provider-egress-proxy#T5.1: empty proxyURL falls back to env proxy
func TestEmptyProxyURLFallbackToEnv(t *testing.T) {
	os.Setenv("HTTP_PROXY", "http://env-proxy:8080")
	defer os.Unsetenv("HTTP_PROXY")

	pf, err := proxyFuncFromURL("")
	if err != nil {
		t.Fatalf("proxyFuncFromURL: %s", err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	proxyURL, err := pf(req)
	if err != nil {
		t.Fatalf("proxy func: %s", err)
	}
	if proxyURL == nil {
		t.Fatal("expected proxy URL from env fallback, got nil")
	}
}

// @sk-test provider-egress-proxy#T5.1: invalid proxy URL returns error
func TestProxyFromInvalidURL(t *testing.T) {
	_, err := proxyFuncFromURL("://invalid")
	if err == nil {
		t.Fatal("expected error for invalid proxy URL")
	}
}

// @sk-test provider-egress-proxy#T5.1: SOCKS5 proxy returns valid transport
func TestNewTransportWithSOCKS5Proxy(t *testing.T) {
	cfg := testConfig()
	tp, err := NewTransport(cfg, "socks5://localhost:1080")
	if err != nil {
		t.Fatalf("NewTransport with socks5: %s", err)
	}
	if tp.DialContext == nil {
		t.Fatal("expected DialContext for SOCKS5, got nil")
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	proxyURL, err := tp.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy: %s", err)
	}
	if proxyURL == nil || !strings.HasPrefix(proxyURL.String(), "socks5://") {
		t.Fatalf("expected socks5 proxy URL, got %v", proxyURL)
	}
}

// @sk-test provider-egress-proxy#T5.1: nil proxy URL in transport
func TestNewTransportWithExplicitNoProxy(t *testing.T) {
	cfg := testConfig()
	tp, err := NewTransport(cfg, "")
	if err != nil {
		t.Fatalf("NewTransport: %s", err)
	}
	if tp.Proxy == nil {
		t.Fatal("expected Proxy func, got nil")
	}
}
