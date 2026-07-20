package health

import (
	"errors"
	"testing"
)

func TestResultStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil error", nil, "ok"},
		{"non-nil error", errors.New("connection failed"), "down"},
		{"typed error", errMockFail, "down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resultStatus(tt.err)
			if got != tt.want {
				t.Errorf("resultStatus(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
