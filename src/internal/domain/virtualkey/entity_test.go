package virtualkey

import (
	"strings"
	"testing"
	"time"
)

// @sk-test 300-virtual-keys#T1.5: Key hash is deterministic SHA-256 hex (AC-001)
func TestKeyHash(t *testing.T) {
	h1 := KeyHash("secret-1")
	h2 := KeyHash("secret-1")
	if h1 != h2 {
		t.Errorf("expected same hash, got %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(h1))
	}
	for _, c := range h1 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("expected hex, got %c", c)
		}
	}
}

// @sk-test 300-virtual-keys#T1.5: Key id and secret use the sk-mc prefix (AC-001)
func TestKeyPrefix(t *testing.T) {
	id, err := NewKeyID()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, KeyPrefix+"_") {
		t.Errorf("expected prefix %s_, got %s", KeyPrefix, id)
	}
	sec, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sec, KeyPrefix+"_") {
		t.Errorf("expected prefix %s_, got %s", KeyPrefix, sec)
	}
	if id == sec {
		t.Error("expected distinct key id and secret")
	}
}

// @sk-test 300-virtual-keys#T1.5: Disabled or expired keys are invalid (AC-001)
func TestValid(t *testing.T) {
	now := time.Now()
	key := fakeTestKey("alpha")
	if !key.Valid(now) {
		t.Error("expected valid key")
	}
	key.Enabled = false
	if key.Valid(now) {
		t.Error("expected disabled key to be invalid")
	}
	key.Enabled = true
	future := now.Add(-time.Hour)
	key.ExpiresAt = &future
	if key.Valid(now) {
		t.Error("expected expired key to be invalid")
	}
}

// @sk-test 300-virtual-keys#T1.5: Allowed/blocked model scopes are enforced (AC-002)
func TestAllowsModel(t *testing.T) {
	open := fakeTestKey("alpha")
	if !open.AllowsModel("gpt-4") {
		t.Error("expected unrestricted key to allow any model")
	}

	allowed := fakeTestKey("alpha")
	allowed.AllowedModels = []string{"gpt-4"}
	if !allowed.AllowsModel("gpt-4") {
		t.Error("expected allowed model to pass")
	}
	if allowed.AllowsModel("gpt-3.5") {
		t.Error("expected non-allowed model to be rejected")
	}

	blocked := fakeTestKey("alpha")
	blocked.BlockedModels = []string{"gpt-4"}
	if blocked.AllowsModel("gpt-4") {
		t.Error("expected blocked model to be rejected")
	}
	if !blocked.AllowsModel("gpt-3.5") {
		t.Error("expected non-blocked model to pass")
	}
}

// @sk-test 300-virtual-keys#T1.5: AddSpend accumulates spend (AC-003)
func TestAddSpend(t *testing.T) {
	key := fakeTestKey("alpha")
	key.AddSpend(1.5)
	key.AddSpend(2.5)
	if key.Spent != 4 {
		t.Errorf("expected spent 4, got %f", key.Spent)
	}
}

func fakeTestKey(tenant string) *VirtualKey {
	return &VirtualKey{
		ID:        "id",
		TenantID:  tenant,
		Label:     "test",
		Metadata:  map[string]string{},
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}
