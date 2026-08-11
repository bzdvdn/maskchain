package analytics

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a cost rate does not exist.
var ErrNotFound = errors.New("cost rate not found")

// @sk-task 150-admin-routing-crud#T1.2: CostRateRepository for cost rate CRUD (AC-002)
//
// CostRateRepository persists per-model cost rates with currency.
type CostRateRepository interface {
	// List returns all cost rates.
	List(ctx context.Context) ([]*CostRate, error)
	// Upsert creates or updates a cost rate, detaching yaml seed (source "ui").
	Upsert(ctx context.Context, rate *CostRate) error
	// Delete removes a cost rate.
	Delete(ctx context.Context, model string) error
	// SeedFromYAML inserts yaml defaults only when the table is empty.
	SeedFromYAML(ctx context.Context, rates []*CostRate) (bool, error)
}
