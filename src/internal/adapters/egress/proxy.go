package egress

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/proxy"
)

// @sk-task 71-egress-streaming#T2.1: Implement proxy resolution from env/config (AC-001)
func proxyFunc() func(*http.Request) (*url.URL, error) {
	httpProxy := resolveEnvVar("HTTP_PROXY")
	httpsProxy := resolveEnvVar("HTTPS_PROXY")
	noProxyRaw := resolveEnvVar("NO_PROXY")

	if httpProxy == "" && httpsProxy == "" {
		return http.ProxyFromEnvironment
	}

	noProxyList := strings.Split(noProxyRaw, ",")
	for i := range noProxyList {
		noProxyList[i] = strings.TrimSpace(noProxyList[i])
	}

	return func(r *http.Request) (*url.URL, error) {
		if r.URL == nil {
			return nil, nil
		}
		host := r.URL.Hostname()
		if isHostInNoProxy(host, noProxyList) {
			return nil, nil
		}
		if r.URL.Scheme == "https" && httpsProxy != "" {
			return url.Parse(httpsProxy)
		}
		if r.URL.Scheme == "http" && httpProxy != "" {
			return url.Parse(httpProxy)
		}
		return http.ProxyFromEnvironment(r)
	}
}

// @sk-task provider-egress-proxy#T2.1: Per-provider proxy from explicit URL
func proxyFuncFromURL(proxyURL string) (func(*http.Request) (*url.URL, error), error) {
	if proxyURL == "" {
		return proxyFunc(), nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	return func(r *http.Request) (*url.URL, error) {
		return parsed, nil
	}, nil
}

// @sk-task provider-egress-proxy#T2.1: SOCKS5 dialer for socks5:// proxy scheme
func socks5DialContext(proxyURL string) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	d, err := proxy.FromURL(parsed, proxy.Direct)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// nolint:wrapcheck
		return d.Dial(network, addr)
	}, nil
}

// @sk-task 71-egress-streaming#T2.1: Add NO_PROXY matching and proxy URL parsing (AC-001)
func isHostInNoProxy(host string, noProxyList []string) bool {
	for _, np := range noProxyList {
		np = strings.TrimSpace(np)
		if np == "" {
			continue
		}
		if np == host || strings.HasSuffix(host, "."+np) {
			return true
		}
	}
	return false
}

func resolveEnvVar(name string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return os.Getenv(strings.ToLower(name))
}
