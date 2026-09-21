package middleware

import (
	"context"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
)

// @sk-task 301-budget-enforcement#T2.3: fakeBudgetRepo for middleware tests (AC-002)
type fakeBudgetRepo struct {
	budgets []*budget.Budget
	spend   []budget.SpendEntry
}

func (f *fakeBudgetRepo) Create(_ context.Context, b *budget.Budget) error {
	f.budgets = append(f.budgets, b)
	return nil
}

func (f *fakeBudgetRepo) Update(_ context.Context, b *budget.Budget) error {
	for i, item := range f.budgets {
		if item.ID == b.ID {
			f.budgets[i] = b
			return nil
		}
	}
	return budget.ErrNotFound
}

func (f *fakeBudgetRepo) Delete(_ context.Context, id string) error {
	for i, b := range f.budgets {
		if b.ID == id {
			f.budgets = append(f.budgets[:i], f.budgets[i+1:]...)
			return nil
		}
	}
	return budget.ErrNotFound
}

func (f *fakeBudgetRepo) GetByID(_ context.Context, id string) (*budget.Budget, error) {
	for _, b := range f.budgets {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, budget.ErrNotFound
}

func (f *fakeBudgetRepo) List(_ context.Context) ([]*budget.Budget, error) {
	return f.budgets, nil
}

// @sk-task ui-production-readiness#T7.1: DB-level pagination/search (AC-008)
func (f *fakeBudgetRepo) ListPaged(_ context.Context, limit, offset int, _ string) ([]*budget.Budget, int, error) {
	total := len(f.budgets)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return f.budgets[start:end], total, nil
}

func (f *fakeBudgetRepo) ListByTenant(_ context.Context, tenantID string) ([]*budget.Budget, error) {
	var out []*budget.Budget
	for _, b := range f.budgets {
		if b.TenantID == tenantID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBudgetRepo) ListActiveForRequest(_ context.Context, tenantID, keyID, model string) ([]*budget.Budget, error) {
	var out []*budget.Budget
	for _, b := range f.budgets {
		if !b.Enabled || b.TenantID != tenantID {
			continue
		}
		switch b.Scope {
		case budget.ScopeKey:
			if b.VirtualKeyID == keyID {
				out = append(out, b)
			}
		case budget.ScopeModel:
			if b.Model == model {
				out = append(out, b)
			}
		default:
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBudgetRepo) RecordSpend(_ context.Context, entry budget.SpendEntry) error {
	f.spend = append(f.spend, entry)
	return nil
}

func (f *fakeBudgetRepo) History(_ context.Context, _ string, _, _ int) ([]budget.SpendEntry, error) {
	return f.spend, nil
}

func (f *fakeBudgetRepo) AggregateDaily(_ context.Context, _ time.Time) error {
	return nil
}

// @sk-task 301-budget-enforcement#T2.3: fakeBudgetCounter for middleware tests (AC-002)
type fakeBudgetCounter struct {
	values map[string]float64
}

func (c *fakeBudgetCounter) Increment(_ context.Context, key string, amount float64, _ time.Duration) (float64, error) {
	if c.values == nil {
		c.values = map[string]float64{}
	}
	c.values[key] += amount
	return c.values[key], nil
}

func (c *fakeBudgetCounter) Current(_ context.Context, key string) (float64, error) {
	return c.values[key], nil
}

func (c *fakeBudgetCounter) Reset(_ context.Context, key string) error {
	if c.values != nil {
		delete(c.values, key)
	}
	return nil
}

// @sk-task 301-budget-enforcement#T2.3: fakeBudgetNotifier records alerts (AC-003)
type fakeBudgetNotifier struct {
	alerts []float64
}

func (n *fakeBudgetNotifier) NotifyBudgetExceeded(_ context.Context, _ *budget.Budget, pct float64, _ float64) error {
	n.alerts = append(n.alerts, pct)
	return nil
}
