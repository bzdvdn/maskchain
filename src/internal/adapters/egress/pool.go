package egress

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// timedConn wraps a net.Conn to log I/O timing.
type timedConn struct {
	net.Conn
	name        string
	connectedAt time.Time
	firstIO     sync.Once
	firstIOAt   time.Time
}

func (tc *timedConn) logFirstIO() {
	tc.firstIO.Do(func() {
		tc.firstIOAt = time.Now()
		elapsed := tc.firstIOAt.Sub(tc.connectedAt)
		// First I/O after connect ≈ TLS handshake complete (for TLS connections)
		if elapsed > 50*time.Millisecond {
			fmt.Fprintf(os.Stderr, "=== CONN FIRST IO (%v) [%s] ===\n", elapsed, tc.name)
		}
	})
}

func (tc *timedConn) Read(b []byte) (int, error) {
	tc.logFirstIO()
	return tc.Conn.Read(b)
}

func (tc *timedConn) Write(b []byte) (int, error) {
	tc.logFirstIO()
	return tc.Conn.Write(b)
}

// preferIPv4DialContext wraps a Dialer with timing and timedConn instrumentation.
// Unlike the original implementation, it does NOT perform a separate DNS lookup
// or force tcp4 — Go 1.26's net.Dialer already implements proper happy eyeballs
// (RFC 8305), trying both A and AAAA records in parallel and using the fastest
// connection. Removing the manual DNS + forced tcp4 avoids:
//   - sequential DNS resolution bottleneck (adds RTT before TCP dial)
//   - loss of IPv6 fast-path when IPv4 is slow or unreachable
//   - per-attempt 30s tcp4 dial timeout with no v6 fallback
func preferIPv4DialContext(d *net.Dialer, debug bool) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if debug {
			fmt.Fprintf(os.Stderr, "=== DIAL CALLED: net=%q addr=%q ===\n", network, addr)
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return d.DialContext(ctx, network, addr)
		}
		t0 := time.Now()
		conn, err := d.DialContext(ctx, network, addr)
		if debug {
			fmt.Fprintf(os.Stderr, "=== DIAL %s done (took=%v, err=%v) ===\n", network, time.Since(t0), err)
		}
		if err != nil {
			return conn, err
		}
		return &timedConn{Conn: conn, name: host, connectedAt: time.Now()}, nil
	}
}

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
		DialContext:         preferIPv4DialContext(dialer, cfg.DebugEnabled),
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleTimeout,
		DisableKeepAlives:   cfg.DisableKeepAlives,
		ForceAttemptHTTP2:   true,
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
