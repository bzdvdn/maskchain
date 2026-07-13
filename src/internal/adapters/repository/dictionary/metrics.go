package dictionaryrepo

import (
	"github.com/prometheus/client_golang/prometheus"
)

type promCacheMetrics struct {
	hits           *prometheus.CounterVec
	misses         *prometheus.CounterVec
	stale          *prometheus.CounterVec
	invalidations  *prometheus.CounterVec
}

func NewPromCacheMetrics(
	hits, misses, stale, invalidations *prometheus.CounterVec,
) *promCacheMetrics {
	return &promCacheMetrics{
		hits:          hits,
		misses:        misses,
		stale:         stale,
		invalidations: invalidations,
	}
}

func (m *promCacheMetrics) IncHits(operation, level string) {
	m.hits.WithLabelValues(operation, level).Inc()
}

func (m *promCacheMetrics) IncMisses(operation, level string) {
	m.misses.WithLabelValues(operation, level).Inc()
}

func (m *promCacheMetrics) IncStale(operation string) {
	m.stale.WithLabelValues(operation).Inc()
}

func (m *promCacheMetrics) IncInvalidations(operation string) {
	m.invalidations.WithLabelValues(operation).Inc()
}
