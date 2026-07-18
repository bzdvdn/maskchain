package egress

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task provider-egress-proxy#T3.1: NewTransport accepts proxyURL for per-provider proxy
// @sk-task 71-egress-streaming#T2.1: Implement connection pool with configurable params (AC-002)
// @sk-task 116-connection-pool-fixes#T2.1: Fix MaxIdleConnsPerHost and wire DisableKeepAlives (AC-001)
// @sk-task 116-connection-pool-fixes#T2.3: Export NewTransport for per-provider usage in factory (AC-008)
// @sk-task 116-connection-pool-fixes#T3.2: Integrate buildTLSConfig into NewTransport (AC-003, AC-004, AC-005)
func NewTransport(cfg *config.EgressConfig, proxyURL string) (*http.Transport, error) {
	dialer := &net.Dialer{
		Timeout:   defaultDialTimeout,
		KeepAlive: 30 * time.Second,
	}

	pf, err := proxyFuncFromURL(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("egress: invalid proxy url: %w", err)
	}

	tp := &http.Transport{
		Proxy:               pf,
		DialContext:         dialer.DialContext,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleTimeout,
		DisableKeepAlives:   cfg.DisableKeepAlives,
		ForceAttemptHTTP2:   false,
	}

	if cfg.TLS != nil {
		tlsCfg, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("egress: build TLS config: %w", err)
		}
		tp.TLSClientConfig = tlsCfg
	}

	if strings.HasPrefix(proxyURL, "socks5://") {
		dialCtx, err := socks5DialContext(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("egress: socks5 dialer: %w", err)
		}
		tp.DialContext = dialCtx
	}

	return tp, nil
}

// @sk-task 116-connection-pool-fixes#T3.2: Implement buildTLSConfig for custom CA, insecure, mTLS (AC-003, AC-004, AC-005)
func buildTLSConfig(cfg *config.EgressTLSConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	if cfg.CACert != "" {
		caCert, err := os.ReadFile(cfg.CACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
		tlsCfg.RootCAs = caPool
	}

	if cfg.Cert != "" || cfg.Key != "" {
		cert, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS cert/key: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
