package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// @sk-task 90-production-hardening#T3.1: PGPoolCollector exposes PG pool stats as Prometheus metrics (<AC-003>)
//
// PGPoolCollector represents a domain entity or configuration.
type PGPoolCollector struct {
	pool         *pgxpool.Pool
	acquireCount *prometheus.Desc
	idleConns    *prometheus.Desc
	inUseConns   *prometheus.Desc
	totalConns   *prometheus.Desc
}

func NewPGPoolCollector(pool *pgxpool.Pool) *PGPoolCollector {
	return &PGPoolCollector{
		pool: pool,
		acquireCount: prometheus.NewDesc(
			namespace+"_pgx_pool_acquire_count",
			"Total number of acquired connections from the pool",
			nil, nil,
		),
		idleConns: prometheus.NewDesc(
			namespace+"_pgx_pool_idle_conns",
			"Number of idle connections in the pool",
			nil, nil,
		),
		inUseConns: prometheus.NewDesc(
			namespace+"_pgx_pool_in_use_conns",
			"Number of in-use (acquired) connections in the pool",
			nil, nil,
		),
		totalConns: prometheus.NewDesc(
			namespace+"_pgx_pool_total_conns",
			"Total number of connections in the pool",
			nil, nil,
		),
	}
}

func (c *PGPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquireCount
	ch <- c.idleConns
	ch <- c.inUseConns
	ch <- c.totalConns
}

func (c *PGPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.acquireCount, prometheus.GaugeValue, float64(stat.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.idleConns, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.inUseConns, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.totalConns, prometheus.GaugeValue, float64(stat.TotalConns()))
}
