package metrics

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// @sk-test 90-production-hardening#T4.3: TestPGPoolCollectorRegistration (<AC-003>)
func TestPGPoolCollectorRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	pool, err := pgxpool.New(context.Background(), "postgres://localhost:5432/test?sslmode=disable")
	if err != nil {
		t.Skip("PG not available for pool creation test:", err)
	}
	defer pool.Close()

	collector := NewPGPoolCollector(pool)
	if collector == nil {
		t.Fatal("expected non-nil collector")
	}

	err = reg.Register(collector)
	if err != nil {
		t.Fatalf("expected no registration error, got: %v", err)
	}
}
