//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-test cleanup-profile-repository#T4.1: TestPoolHealthcheck (AC-005)
func TestPoolHealthcheck(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool ping: %v", err)
	}
}

func setupPG(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("SHIELD_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("SHIELD_TEST_PG_DSN not set; skipping integration test")
	}

	cfg := &config.DatabaseConfig{
		DSN:             dsn,
		MaxConns:        5,
		MinConns:        1,
		MaxConnLifetime: 30 * time.Minute,
	}

	pool, err := NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}

	if err := RunMigrations(dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return pool
}
