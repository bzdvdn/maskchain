package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// @sk-task 30-shield-persistence#T2.1: Implement TransactionManager interface (DEC-002)
type TransactionManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// @sk-task 30-shield-persistence#T2.1: Implement PGXTransactionManager (DEC-002)
type PGXTransactionManager struct {
	pool *pgxpool.Pool
}

func NewPGXTransactionManager(pool *pgxpool.Pool) *PGXTransactionManager {
	return &PGXTransactionManager{pool: pool}
}

func (tm *PGXTransactionManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey{}, tx)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback error: %v (original: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

type txKey struct{}

// @sk-task 30-shield-persistence#T2.1: Add querier helper for tx-aware queries
func getQuerier(ctx context.Context, pool *pgxpool.Pool) querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
