package budget

import "context"

// @sk-task rate-limiting-budgets#T1.2: Create TokenBudget value object (AC-002, AC-005)
type TokenBudget struct {
	Budget    int64
	Remaining int64
	Model     string
}

// @sk-task rate-limiting-budgets#T1.2: Create TokenBudgetRepository interface (AC-002, AC-005)
type TokenBudgetRepository interface {
	Remaining(ctx context.Context, budgetKey string, budgetLimit int64) (int64, error)
	Deduct(ctx context.Context, budgetKey string, tokens int64, ttlSec int64) (int64, error)
	Reset(ctx context.Context, budgetKey string) error
}
