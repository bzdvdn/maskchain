package version

import (
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	info := Info()
	if !strings.HasPrefix(info, "MaskChain ") {
		t.Errorf("expected prefix 'MaskChain ', got %q", info)
	}
}

func TestInfoWithValues(t *testing.T) {
	Version = "1.0.0"
	Commit = "abc1234"
	Date = "2025-01-01T00:00:00Z"

	info := Info()
	if !strings.Contains(info, "1.0.0") {
		t.Errorf("expected version 1.0.0 in info, got %q", info)
	}
	if !strings.Contains(info, "abc1234") {
		t.Errorf("expected commit abc1234 in info, got %q", info)
	}
	if !strings.Contains(info, "2025-01-01T00:00:00Z") {
		t.Errorf("expected date in info, got %q", info)
	}
}
