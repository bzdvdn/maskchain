package egress

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// @sk-task 71-egress-streaming#T3.1: Implement retry with exponential backoff, full jitter, exhaustion (AC-006, AC-007)
func backoff(attempt int, base time.Duration) time.Duration {
	max := base * (1 << attempt)
	if max <= 0 {
		return base
	}
	return time.Duration(rand.Int64N(int64(max)))
}

// @sk-task 71-egress-streaming#T3.1: Implement retry policy (AC-006, AC-007)
func isRetriable(err error, statusCode int, method string, retryOn5xx bool) bool {
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return true
		}
		if strings.Contains(err.Error(), "connection refused") {
			return true
		}
		return false
	}
	if statusCode >= 500 {
		return method == http.MethodGet || retryOn5xx
	}
	return false
}

// @sk-task 71-egress-streaming#T3.1: Implement retry loop with context cancellation (AC-005, AC-007)
func (c *Client) doWithRetry(ctx context.Context, method string, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	maxRetries := c.cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			wait := backoff(attempt, c.cfg.BaseBackoff)
			if c.cfg.DebugEnabled {
				fmt.Fprintf(os.Stderr, "=== RETRY backoff=%v (attempt=%d/%d) ===\n", wait, attempt, maxRetries)
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		attemptStart := time.Now()
		resp, err := fn()
		if c.cfg.DebugEnabled {
			attemptDur := time.Since(attemptStart)
			if err != nil {
				fmt.Fprintf(os.Stderr, "=== ATTEMPT %d/%d FAILED (dur=%v, err=%v) ===\n", attempt, maxRetries, attemptDur, err)
			} else if resp.StatusCode >= 500 {
				fmt.Fprintf(os.Stderr, "=== ATTEMPT %d/%d 5xx (dur=%v, status=%d) ===\n", attempt, maxRetries, attemptDur, resp.StatusCode)
			} else {
				fmt.Fprintf(os.Stderr, "=== ATTEMPT %d/%d OK (dur=%v) ===\n", attempt, maxRetries, attemptDur)
			}
		}
		if err != nil {
			if attempt < maxRetries && isRetriable(err, 0, method, c.cfg.RetryOn5xx) {
				lastErr = err
				continue
			}
			return nil, err
		}

		if resp.StatusCode >= 500 {
			if isRetriable(nil, resp.StatusCode, method, c.cfg.RetryOn5xx) {
				resp.Body.Close()
				if attempt < maxRetries {
					lastErr = errFromStatus(resp.StatusCode)
					continue
				}
				return nil, errFromStatus(resp.StatusCode)
			}
			return resp, nil
		}

		return resp, nil
	}

	return nil, lastErr
}

func errFromStatus(statusCode int) error {
	return &retryError{status: statusCode}
}

type retryError struct {
	status int
}

func (e *retryError) Error() string {
	return "retry exhausted: " + http.StatusText(e.status)
}
