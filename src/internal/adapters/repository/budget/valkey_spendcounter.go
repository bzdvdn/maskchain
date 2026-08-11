package budgetrepo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"
)

// @sk-task 301-budget-enforcement#T1.2: ValkeySpendCounter implements SpendCounter via INCRBYFLOAT (AC-002)
//
// ValkeySpendCounter is a fast atomic budget counter keyed by period.
type ValkeySpendCounter struct {
	client valkey.Client
}

func NewValkeySpendCounter(client valkey.Client) *ValkeySpendCounter {
	return &ValkeySpendCounter{client: client}
}

// @sk-task 301-budget-enforcement#T1.2: Increment adds amount and returns the new total (AC-002)
func (c *ValkeySpendCounter) Increment(ctx context.Context, key string, amount float64, ttl time.Duration) (float64, error) {
	if c.client == nil {
		return 0, nil
	}
	resp := c.client.Do(ctx, c.client.B().Incrbyfloat().Key(key).Increment(amount).Build())
	total, err := resp.AsFloat64()
	if err != nil {
		return 0, fmt.Errorf("budget counter incrbyfloat: %w", err)
	}
	if ttl > 0 {
		_ = c.client.Do(ctx, c.client.B().Expire().Key(key).Seconds(int64(ttl.Seconds())).Build()).Error()
	}
	return total, nil
}

// @sk-task 301-budget-enforcement#T1.2: Current returns the current counter value (AC-002)
func (c *ValkeySpendCounter) Current(ctx context.Context, key string) (float64, error) {
	if c.client == nil {
		return 0, nil
	}
	val, err := c.client.Do(ctx, c.client.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("budget counter get: %w", err)
	}
	total, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("budget counter parse: %w", err)
	}
	return total, nil
}

// @sk-task 301-budget-enforcement#T1.2: Reset deletes the counter (AC-002)
func (c *ValkeySpendCounter) Reset(ctx context.Context, key string) error {
	if c.client == nil {
		return nil
	}
	return c.client.Do(ctx, c.client.B().Del().Key(key).Build()).Error()
}
