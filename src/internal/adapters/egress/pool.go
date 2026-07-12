package egress

import (
	"net"
	"net/http"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 71-egress-streaming#T2.1: Implement connection pool with configurable params (AC-002)
func newTransport(cfg *config.EgressConfig) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   defaultDialTimeout,
		KeepAlive: 30 * time.Second,
	}

	tp := &http.Transport{
		Proxy:               proxyFunc(),
		DialContext:         dialer.DialContext,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConns,
		IdleConnTimeout:     cfg.IdleTimeout,
		ForceAttemptHTTP2:   false,
	}

	return tp
}
