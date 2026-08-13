package cache

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCacheKeyDeterministic(t *testing.T) {
	embedding := []float32{0.1, 0.2, -0.3, 1.0}
	k1, err := CacheKey("tenant-a", embedding)
	if err != nil {
		t.Fatalf("CacheKey: %v", err)
	}
	k2, err := CacheKey("tenant-a", embedding)
	if err != nil {
		t.Fatalf("CacheKey: %v", err)
	}
	if k1 != k2 {
		t.Fatalf("expected deterministic key, got %q vs %q", k1, k2)
	}
}

// @sk-task semantic-cache-masked#T3.3: tenant isolation in key (AC-003)
func TestCacheKeyTenantIsolation(t *testing.T) {
	embedding := []float32{0.1, 0.2, -0.3, 1.0}
	ka, err := CacheKey("tenant-a", embedding)
	if err != nil {
		t.Fatalf("CacheKey: %v", err)
	}
	kb, err := CacheKey("tenant-b", embedding)
	if err != nil {
		t.Fatalf("CacheKey: %v", err)
	}
	if ka == kb {
		t.Fatalf("expected different keys per tenant, both %q", ka)
	}
	if !strings.HasPrefix(ka, KeyPrefixCache+"tenant-a:") {
		t.Fatalf("expected prefix cache:tenant-a:, got %q", ka)
	}
	if !strings.HasPrefix(kb, KeyPrefixCache+"tenant-b:") {
		t.Fatalf("expected prefix cache:tenant-b:, got %q", kb)
	}
}

// @sk-task semantic-cache-masked#T3.4: no raw data in keys (AC-004)
func TestCacheKeyContainsNoRaw(t *testing.T) {
	embedding := []float32{0.1, 0.2, -0.3, 1.0}
	k, err := CacheKey("tenant-a", embedding)
	if err != nil {
		t.Fatalf("CacheKey: %v", err)
	}
	for _, raw := range []string{"0.1", "0.2", "-0.3"} {
		if strings.Contains(k, raw) {
			t.Fatalf("key %q leaks raw embedding value %q", k, raw)
		}
	}
}

func TestCacheKeyRejectsEmpty(t *testing.T) {
	if _, err := CacheKey("", []float32{1.0}); err == nil {
		t.Fatal("expected error for empty tenant")
	}
	if _, err := CacheKey("tenant-a", nil); err == nil {
		t.Fatal("expected error for empty embedding")
	}
}

func TestCacheEntryRoundTrip(t *testing.T) {
	payload := []byte(`{"choices":[{"message":{"role":"assistant","content":"mocked"}}]}`)
	entry := NewCacheEntry(payload, time.Hour)
	data, err := entry.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	decoded, err := UnmarshalCacheEntry(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Fatalf("payload mismatch: got %q want %q", decoded.Payload, payload)
	}
	if !decoded.CreatedAt.Equal(entry.CreatedAt) {
		t.Fatalf("created_at mismatch: got %v want %v", decoded.CreatedAt, entry.CreatedAt)
	}
	if !decoded.ExpiresAt.Equal(entry.ExpiresAt) {
		t.Fatalf("expires_at mismatch: got %v want %v", decoded.ExpiresAt, entry.ExpiresAt)
	}
}

// @sk-task semantic-cache-masked#T3.4: no raw data in values (AC-005)
func TestCacheEntryMarshalLeaksNoRaw(t *testing.T) {
	raw := "shh-secret-api-key-or-pii"
	payload := []byte(`{"content":"` + raw + `"}`)
	entry := NewCacheEntry(payload, time.Hour)
	data, err := entry.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// base64 of payload must not embed the raw substring directly.
	if bytes.Contains(data, []byte(raw)) {
		t.Fatalf("entry bytes leak raw payload: %s", data)
	}
}

func TestCacheEntryExpired(t *testing.T) {
	entry := NewCacheEntry([]byte(`{}`), time.Minute)
	now := entry.CreatedAt
	if entry.Expired(now) {
		t.Fatal("fresh entry must not be expired")
	}
	if !entry.Expired(now.Add(2 * time.Minute)) {
		t.Fatal("entry must be expired past TTL")
	}
}

func TestCacheEntryJSONShape(t *testing.T) {
	entry := NewCacheEntry([]byte(`{"ok":true}`), time.Hour)
	data, err := entry.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("entry is not valid JSON: %v", err)
	}
	for _, k := range []string{"payload", "created_at", "expires_at"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("entry JSON missing key %q: %s", k, data)
		}
	}
}
