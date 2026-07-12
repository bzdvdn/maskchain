package budgetrepo

import (
	"context"
	"fmt"
	"strconv"

	"github.com/valkey-io/valkey-go"
)

// @sk-task rate-limiting-budgets#T3.5: Implement ValkeyTokenBudgetRepo (AC-002, AC-005)
type ValkeyTokenBudgetRepo struct {
	client valkey.Client
}

func NewValkeyTokenBudgetRepo(client valkey.Client) *ValkeyTokenBudgetRepo {
	return &ValkeyTokenBudgetRepo{client: client}
}

// @sk-task rate-limiting-budgets#T3.5: Implement Remaining with GET (AC-002, AC-005)
func (r *ValkeyTokenBudgetRepo) Remaining(ctx context.Context, budgetKey string, budgetLimit int64) (int64, error) {
	if r.client == nil {
		return budgetLimit, nil
	}
	val, err := r.client.Do(ctx, r.client.B().Get().Key(budgetKey).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return budgetLimit, nil
		}
		return budgetLimit, fmt.Errorf("token budget get: %w", err)
	}
	used, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return budgetLimit, nil
	}
	remaining := budgetLimit - used
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// @sk-task rate-limiting-budgets#T3.5: Implement Deduct with INCRBY+EXPIRE (AC-002, AC-005)
func (r *ValkeyTokenBudgetRepo) Deduct(ctx context.Context, budgetKey string, tokens int64, ttlSec int64) (int64, error) {
	if r.client == nil {
		return 0, nil
	}
	used, err := r.client.Do(ctx, r.client.B().Incrby().Key(budgetKey).Increment(tokens).Build()).AsInt64()
	if err != nil {
		return 0, fmt.Errorf("token budget incrby: %w", err)
	}
	_ = r.client.Do(ctx, r.client.B().Expire().Key(budgetKey).Seconds(ttlSec).Build()).Error()
	return used, nil
}

func (r *ValkeyTokenBudgetRepo) Reset(ctx context.Context, budgetKey string) error {
	if r.client == nil {
		return nil
	}
	return r.client.Do(ctx, r.client.B().Del().Key(budgetKey).Build()).Error()
}
