package egress

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestIsRetriable(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		method     string
		retryOn5xx bool
		want       bool
	}{
		{
			name: "network timeout",
			err:  &net.DNSError{IsTimeout: true},
			want: true,
		},
		{
			name: "connection refused",
			err:  errors.New("connection refused"),
			want: true,
		},
		{
			name: "non-retriable error",
			err:  errors.New("invalid request"),
			want: false,
		},
		{
			name:       "5xx with retryOn5xx",
			statusCode: 503,
			method:     "POST",
			retryOn5xx: true,
			want:       true,
		},
		{
			name:       "5xx without retryOn5xx POST",
			statusCode: 503,
			method:     "POST",
			retryOn5xx: false,
			want:       false,
		},
		{
			name:       "5xx without retryOn5xx GET",
			statusCode: 503,
			method:     "GET",
			retryOn5xx: false,
			want:       true,
		},
		{
			name:       "4xx not retriable",
			statusCode: 429,
			method:     "GET",
			retryOn5xx: false,
			want:       false,
		},
		{
			name:       "200 not retriable",
			statusCode: 200,
			want:       false,
		},
		{
			name: "nil error and no status",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetriable(tt.err, tt.statusCode, tt.method, tt.retryOn5xx)
			if got != tt.want {
				t.Errorf("isRetriable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	base := 100 * time.Millisecond

	for attempt := 0; attempt < 5; attempt++ {
		d := backoff(attempt, base)
		if d < 0 {
			t.Errorf("backoff(%d, %v) = %v, expected >= 0", attempt, base, d)
		}
		maxDelay := base * (1 << attempt)
		if d > maxDelay {
			t.Errorf("backoff(%d, %v) = %v, expected <= %v", attempt, base, d, maxDelay)
		}
	}
}

func TestErrFromStatus(t *testing.T) {
	err := errFromStatus(503)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Error() != "retry exhausted: Service Unavailable" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}
