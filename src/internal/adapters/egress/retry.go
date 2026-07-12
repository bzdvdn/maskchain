package egress

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"net/http"
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
	attempts := c.cfg.MaxRetries
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 0; attempt <= attempts; attempt++ {
		if attempt > 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			wait := backoff(attempt, c.cfg.BaseBackoff)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := fn()
		if err != nil {
			if attempt < attempts && isRetriable(err, 0, method, c.cfg.RetryOn5xx) {
				lastErr = err
				continue
			}
			return nil, err
		}

		if resp.StatusCode >= 500 {
			if isRetriable(nil, resp.StatusCode, method, c.cfg.RetryOn5xx) {
				resp.Body.Close()
				if attempt < attempts {
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
