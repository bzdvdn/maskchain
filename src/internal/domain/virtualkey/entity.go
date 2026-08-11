package virtualkey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a virtual key does not exist.
var ErrNotFound = errors.New("virtual key not found")

// KeyPrefix is the visible prefix of every generated key.
const KeyPrefix = "sk-mc"

// KeyHash returns a constant-time-friendly SHA-256 digest of a raw key.
func KeyHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// @sk-task 300-virtual-keys#T1.1: VirtualKey entity — universal tenant key with scopes (AC-001)
type VirtualKey struct {
	ID            string
	TenantID      string
	KeyHash       string
	Label         string
	AllowedModels []string
	BlockedModels []string
	BudgetCap     *float64
	Spent         float64
	ExpiresAt     *time.Time
	Metadata      map[string]string
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// @sk-task 300-virtual-keys#T1.1: NewKeyID generates a new public key id
func NewKeyID() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate key id: %w", err)
	}
	return KeyPrefix + "_" + hex.EncodeToString(b), nil
}

// @sk-task 300-virtual-keys#T1.1: NewSecret generates a plaintext secret shown once (AC-001)
func NewSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return KeyPrefix + "_" + hex.EncodeToString(b), nil
}

// @sk-task 300-virtual-keys#T1.1: Valid reports whether the key is enabled and not expired (AC-001)
func (k *VirtualKey) Valid(now time.Time) bool {
	if !k.Enabled {
		return false
	}
	if k.ExpiresAt != nil && now.After(*k.ExpiresAt) {
		return false
	}
	return true
}

// @sk-task 300-virtual-keys#T1.1: AllowsModel evaluates allowed/blocked model scopes (AC-002)
func (k *VirtualKey) AllowsModel(model string) bool {
	if len(k.AllowedModels) == 0 && len(k.BlockedModels) == 0 {
		return true
	}
	if contains(k.BlockedModels, model) {
		return false
	}
	if len(k.AllowedModels) > 0 && !contains(k.AllowedModels, model) {
		return false
	}
	return true
}

// @sk-task 300-virtual-keys#T1.1: AddSpend increments the cumulative spend for the key (AC-003)
func (k *VirtualKey) AddSpend(amount float64) {
	k.Spent += amount
}

func contains(items []string, v string) bool {
	for _, item := range items {
		if item == v {
			return true
		}
	}
	return false
}
