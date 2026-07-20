package budget

import "context"

// @sk-task rate-limiting-budgets#T1.2: Create RateLimit value object (AC-001, AC-004)
//
// RateLimit represents a domain entity or configuration.
type RateLimit struct {
	Allowed   bool
	Limit     int64
	Remaining int64
	ResetTime int64
}

// @sk-task rate-limiting-budgets#T1.2: Create RateLimitRepository interface (AC-001, AC-004)
//
// RateLimitRepository defines the interface for domain operations.
type RateLimitRepository interface {
	Allow(ctx context.Context, windowKey string, limit int64, windowSec int64) (*RateLimit, error)
	Reset(ctx context.Context, windowKey string) error
}
