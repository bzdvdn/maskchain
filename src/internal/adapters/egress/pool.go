package egress

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 71-egress-streaming#T2.1: Implement connection pool with configurable params (AC-002)
// @sk-task 116-connection-pool-fixes#T2.1: Fix MaxIdleConnsPerHost and wire DisableKeepAlives (AC-001)
// @sk-task 116-connection-pool-fixes#T2.3: Export NewTransport for per-provider usage in factory (AC-008)
// @sk-task 116-connection-pool-fixes#T3.2: Integrate buildTLSConfig into NewTransport (AC-003, AC-004, AC-005)
func NewTransport(cfg *config.EgressConfig) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   defaultDialTimeout,
		KeepAlive: 30 * time.Second,
	}

	tp := &http.Transport{
		Proxy:               proxyFunc(),
		DialContext:         dialer.DialContext,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleTimeout,
		DisableKeepAlives:   cfg.DisableKeepAlives,
		ForceAttemptHTTP2:   false,
	}

	if cfg.TLS != nil {
		tp.TLSClientConfig = buildTLSConfig(cfg.TLS)
	}

	return tp
}

// @sk-task 116-connection-pool-fixes#T3.2: Implement buildTLSConfig for custom CA, insecure, mTLS (AC-003, AC-004, AC-005)
func buildTLSConfig(cfg *config.EgressTLSConfig) *tls.Config {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	if cfg.CACert != "" {
		caCert, err := os.ReadFile(cfg.CACert)
		if err != nil {
			panic("egress: failed to read CA cert: " + err.Error())
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			panic("egress: failed to parse CA cert")
		}
		tlsCfg.RootCAs = caPool
	}

	if cfg.Cert != "" || cfg.Key != "" {
		cert, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
		if err != nil {
			panic("egress: failed to load TLS cert/key: " + err.Error())
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg
}
