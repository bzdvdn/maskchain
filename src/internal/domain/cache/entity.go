package cache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"time"
)

// @sk-task semantic-cache-masked#T1.1: CacheEntry serialization + expiry (AC-001, AC-004)
//
// CacheEntry is the value stored for a masked response. It never contains raw
// provider payloads: the payload is the masked JSON body only.
type CacheEntry struct {
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewCacheEntry builds an entry with the given TTL window.
func NewCacheEntry(payload []byte, ttl time.Duration) CacheEntry {
	now := time.Now().UTC()
	return CacheEntry{
		Payload:   payload,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
}

// Expired reports whether the entry is past its expiry time.
func (e CacheEntry) Expired(now time.Time) bool {
	return now.After(e.ExpiresAt)
}

// Marshal serializes the entry for storage.
func (e CacheEntry) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalCacheEntry decodes a stored entry.
func UnmarshalCacheEntry(data []byte) (CacheEntry, error) {
	var e CacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return CacheEntry{}, err
	}
	return e, nil
}

// @sk-task semantic-cache-masked#T1.1: cache key derivation (AC-003, AC-005)
//
// CacheKey derives the Valkey key for a tenant + masked-embedding pair as
// cache:<tenant>:<sha256(masked-embedding)>. The key is deterministic and
// embeds the tenant so cache entries are isolated per tenant.
func CacheKey(tenantID string, embedding []float32) (string, error) {
	if tenantID == "" {
		return "", errors.New("tenant id must not be empty")
	}
	if len(embedding) == 0 {
		return "", errors.New("embedding must not be empty")
	}
	h := sha256.New()
	buf := make([]byte, 4)
	for _, v := range embedding {
		binary.LittleEndian.PutUint32(buf, math.Float32bits(v))
		_, _ = h.Write(buf)
	}
	return KeyPrefixCache + tenantID + ":" + hex.EncodeToString(h.Sum(nil)), nil
}
