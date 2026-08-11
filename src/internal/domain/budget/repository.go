package budget

import (
	"context"
	"time"
)

// @sk-task 301-budget-enforcement#T1.2: BudgetRepository for budget CRUD and spend history (AC-002)
//
// BudgetRepository persists budgets and spend history.
type BudgetRepository interface {
	Create(ctx context.Context, b *Budget) error
	Update(ctx context.Context, b *Budget) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*Budget, error)
	List(ctx context.Context) ([]*Budget, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*Budget, error)
	// ListActiveForRequest returns enabled budgets that match the request context
	// (tenant and/or key and/or model scope).
	ListActiveForRequest(ctx context.Context, tenantID, keyID, model string) ([]*Budget, error)
	// RecordSpend appends a spend entry to the budget history.
	RecordSpend(ctx context.Context, entry SpendEntry) error
	// History returns spend entries for a budget, newest first.
	History(ctx context.Context, budgetID string, limit, offset int) ([]SpendEntry, error)
	// AggregateDaily upserts materialized daily spend for a calendar day.
	AggregateDaily(ctx context.Context, day time.Time) error
}

// @sk-task 301-budget-enforcement#T1.2: SpendEntry represents a single recorded spend (AC-002)
//
// SpendEntry represents a domain entity or configuration.
type SpendEntry struct {
	ID           string
	BudgetID     string
	VirtualKeyID string
	TenantID     string
	Model        string
	Cost         float64
	Tokens       int64
	CreatedAt    time.Time
}

// @sk-task 301-budget-enforcement#T1.2: SpendCounter increments budget spend in a fast store (AC-002)
//
// SpendCounter provides atomic budget counters keyed by budget period.
type SpendCounter interface {
	// Increment adds amount to the counter and returns the new total.
	Increment(ctx context.Context, key string, amount float64, ttl time.Duration) (float64, error)
	// Current returns the current counter value (0 when unset).
	Current(ctx context.Context, key string) (float64, error)
	Reset(ctx context.Context, key string) error
}

// @sk-task 301-budget-enforcement#T1.3: AlertNotifier delivers budget threshold alerts (AC-003)
//
// AlertNotifier delivers budget threshold alerts.
type AlertNotifier interface {
	// NotifyBudgetExceeded fires when a budget crosses a notify_at percentage.
	NotifyBudgetExceeded(ctx context.Context, b *Budget, pct float64, spent float64) error
}
