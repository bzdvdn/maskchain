package budget

import "testing"

// @sk-test release-readiness: budget domain entity tests

func TestKeyPrefixes(t *testing.T) {
	if KeyPrefixRateLimit != "ratelimit:" {
		t.Errorf("KeyPrefixRateLimit = %q, want %q", KeyPrefixRateLimit, "ratelimit:")
	}
	if KeyPrefixTokenBudget != "tokenbudget:" {
		t.Errorf("KeyPrefixTokenBudget = %q, want %q", KeyPrefixTokenBudget, "tokenbudget:")
	}
}

func TestRateLimit_ZeroValue(t *testing.T) {
	var rl RateLimit
	if rl.Allowed {
		t.Error("expected Allowed to be false")
	}
	if rl.Limit != 0 {
		t.Errorf("Limit = %d, want 0", rl.Limit)
	}
	if rl.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", rl.Remaining)
	}
	if rl.ResetTime != 0 {
		t.Errorf("ResetTime = %d, want 0", rl.ResetTime)
	}
}

func TestRateLimit_Values(t *testing.T) {
	rl := RateLimit{
		Allowed:   true,
		Limit:     100,
		Remaining: 42,
		ResetTime: 1712345678,
	}
	if !rl.Allowed {
		t.Error("expected Allowed to be true")
	}
	if rl.Limit != 100 {
		t.Errorf("Limit = %d, want 100", rl.Limit)
	}
	if rl.Remaining != 42 {
		t.Errorf("Remaining = %d, want 42", rl.Remaining)
	}
	if rl.ResetTime != 1712345678 {
		t.Errorf("ResetTime = %d, want 1712345678", rl.ResetTime)
	}
}

func TestTokenBudget_ZeroValue(t *testing.T) {
	var tb TokenBudget
	if tb.Budget != 0 {
		t.Errorf("Budget = %d, want 0", tb.Budget)
	}
	if tb.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", tb.Remaining)
	}
	if tb.Model != "" {
		t.Errorf("Model = %q, want empty", tb.Model)
	}
}

func TestTokenBudget_Values(t *testing.T) {
	tb := TokenBudget{
		Budget:    1000000,
		Remaining: 500000,
		Model:     "gpt-4",
	}
	if tb.Budget != 1000000 {
		t.Errorf("Budget = %d, want 1000000", tb.Budget)
	}
	if tb.Remaining != 500000 {
		t.Errorf("Remaining = %d, want 500000", tb.Remaining)
	}
	if tb.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", tb.Model, "gpt-4")
	}
}
