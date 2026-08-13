package cacheapp

import (
	"context"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
)

// @sk-task semantic-cache-masked#T3.2: BudgetWriteGuard budget-safe writes (AC-008)
//
// BudgetWriteGuard blocks new cache writes once a tenant's spend approaches the
// hard limit, leaving the residual budget for un-cached requests. Hits are
// never affected: the guard only gates writes.
type BudgetWriteGuard struct {
	repo         budget.BudgetRepository
	counter      budget.SpendCounter
	guardPercent float64
}

func NewBudgetWriteGuard(repo budget.BudgetRepository, counter budget.SpendCounter, guardPercent float64) *BudgetWriteGuard {
	return &BudgetWriteGuard{repo: repo, counter: counter, guardPercent: guardPercent}
}

// @sk-task semantic-cache-masked#T3.2: CanWrite checks spend against the guard (AC-008)
func (g *BudgetWriteGuard) CanWrite(ctx context.Context, tenantID string) bool {
	if g == nil || g.repo == nil || g.counter == nil {
		return true
	}
	matched, err := g.repo.ListActiveForRequest(ctx, tenantID, "", "")
	if err != nil || len(matched) == 0 {
		return true
	}
	guardFactor := g.guardPercent
	if guardFactor < 0 || guardFactor > 100 {
		guardFactor = 5
	}
	limit := 1 - guardFactor/100
	now := time.Now().UTC()
	for _, b := range matched {
		if b.HardLimit == nil {
			continue
		}
		spent, err := g.counter.Current(ctx, b.CounterKey(now))
		if err != nil {
			continue
		}
		if spent >= *b.HardLimit*limit {
			return false
		}
	}
	return true
}

// WriteGuard lets the middleware decide whether storing is allowed.
type WriteGuard interface {
	CanWrite(ctx context.Context, tenantID string) bool
}
