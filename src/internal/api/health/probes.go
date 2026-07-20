package health

import (
	"context"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"
)

// @sk-task 114-real-health-probes#T3.1: PGProbe implements Probe for PostgreSQL ping (AC-002, AC-003, AC-004)
//
// PGProbe represents a domain entity or configuration.
type PGProbe struct {
	pool *pgxpool.Pool
}

func NewPGProbe(pool *pgxpool.Pool) *PGProbe {
	return &PGProbe{pool: pool}
}

func (p *PGProbe) Name() string {
	return "database"
}

func (p *PGProbe) Check(ctx context.Context) Result {
	if p.pool == nil {
		return NewResult("ok", 0, nil)
	}
	start := time.Now()
	err := p.pool.Ping(ctx)
	return NewResult(resultStatus(err), time.Since(start), err)
}

// @sk-task 114-real-health-probes#T3.1: ValkeyProbe implements Probe for Valkey PING (AC-002, AC-003)
//
// ValkeyProbe represents a domain entity or configuration.
type ValkeyProbe struct {
	client valkey.Client
}

func NewValkeyProbe(client valkey.Client) *ValkeyProbe {
	return &ValkeyProbe{client: client}
}

func (p *ValkeyProbe) Name() string {
	return "valkey"
}

func (p *ValkeyProbe) Check(ctx context.Context) Result {
	if p.client == nil {
		return NewResult("ok", 0, nil)
	}
	start := time.Now()
	err := p.client.Do(ctx, p.client.B().Ping().Build()).Error()
	return NewResult(resultStatus(err), time.Since(start), err)
}

// @sk-task 114-real-health-probes#T3.1: EgressProbe implements Probe for TCP dial to providers (AC-002)
//
// EgressProbe represents a domain entity or configuration.
type EgressProbe struct {
	targets []string
}

func NewEgressProbe(targets []string) *EgressProbe {
	return &EgressProbe{targets: targets}
}

func (p *EgressProbe) Name() string {
	return "egress"
}

func (p *EgressProbe) Check(ctx context.Context) Result {
	if len(p.targets) == 0 {
		return NewResult("ok", 0, nil)
	}
	start := time.Now()
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	for _, target := range p.targets {
		conn, err := dialer.DialContext(ctx, "tcp", target)
		if err == nil {
			conn.Close()
			return NewResult("ok", time.Since(start), nil)
		}
	}
	return NewResult("down", time.Since(start), nil)
}

func resultStatus(err error) string {
	if err != nil {
		return "down"
	}
	return "ok"
}
