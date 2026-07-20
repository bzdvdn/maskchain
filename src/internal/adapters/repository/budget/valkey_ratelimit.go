package budgetrepo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
)

const slidingWindowScriptStr = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2]) * 1000
local now = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window_ms)

local count = redis.call('ZCARD', key)

if count >= limit then
	local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
	local reset_time = 0
	if #oldest >= 2 then
		reset_time = oldest[2] + window_ms
	end
	return {0, reset_time, limit}
end

redis.call('ZADD', key, now, member)
redis.call('EXPIRE', key, ARGV[2])

local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local reset_time = now + window_ms
if #oldest >= 2 then
	reset_time = oldest[2] + window_ms
end

local remaining = limit - count - 1
return {remaining, reset_time, limit}
`

// @sk-task rate-limiting-budgets#T2.1: Implement ValkeyRateLimitRepo with sliding window (AC-001, AC-004)
//
// ValkeyRateLimitRepo represents a domain entity or configuration.
type ValkeyRateLimitRepo struct {
	client valkey.Client
}

func NewValkeyRateLimitRepo(client valkey.Client) *ValkeyRateLimitRepo {
	return &ValkeyRateLimitRepo{client: client}
}

func randomMember() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// @sk-task rate-limiting-budgets#T2.1: Implement Allow with atomic Lua script (AC-001, AC-004)
func (r *ValkeyRateLimitRepo) Allow(ctx context.Context, windowKey string, limit int64, windowSec int64) (*budget.RateLimit, error) {
	if r.client == nil {
		return &budget.RateLimit{Allowed: true, Limit: limit, Remaining: limit, ResetTime: 0}, nil
	}

	if limit <= 0 {
		return &budget.RateLimit{Allowed: true, Limit: limit, Remaining: limit, ResetTime: 0}, nil
	}

	now := time.Now().UnixMilli()
	member := randomMember()

	resp := r.client.Do(ctx, r.client.B().Eval().Script(slidingWindowScriptStr).Numkeys(1).
		Key(windowKey).
		Arg(fmt.Sprintf("%d", limit)).
		Arg(fmt.Sprintf("%d", windowSec)).
		Arg(fmt.Sprintf("%d", now)).
		Arg(member).
		Build())

	arr, err := resp.ToArray()
	if err != nil {
		return nil, fmt.Errorf("rate limit lua exec: %w", err)
	}

	if len(arr) < 3 {
		return nil, fmt.Errorf("rate limit lua: unexpected result length %d", len(arr))
	}

	remaining, err := arr[0].AsInt64()
	if err != nil {
		return nil, fmt.Errorf("rate limit lua remaining: %w", err)
	}
	resetTime, err := arr[1].AsInt64()
	if err != nil {
		return nil, fmt.Errorf("rate limit lua reset_time: %w", err)
	}
	rlLimit, err := arr[2].AsInt64()
	if err != nil {
		return nil, fmt.Errorf("rate limit lua limit: %w", err)
	}

	return &budget.RateLimit{
		Allowed:   remaining > 0,
		Limit:     rlLimit,
		Remaining: remaining,
		ResetTime: resetTime,
	}, nil
}

func (r *ValkeyRateLimitRepo) Reset(ctx context.Context, windowKey string) error {
	if r.client == nil {
		return nil
	}
	return r.client.Do(ctx, r.client.B().Del().Key(windowKey).Build()).Error()
}
