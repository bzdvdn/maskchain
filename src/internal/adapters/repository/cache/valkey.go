package cacherepo

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

// @sk-task semantic-cache-masked#T1.2: ValkeyCacheStore adapter (AC-004, AC-001)
//
// ValkeyCacheStore implements cache.Store on top of Valkey with a write size
// guard: entries larger than maxEntryBytes are silently skipped.
type ValkeyCacheStore struct {
	client        valkey.Client
	maxEntryBytes int64
}

func NewValkeyCacheStore(client valkey.Client, maxEntryBytes int64) *ValkeyCacheStore {
	return &ValkeyCacheStore{client: client, maxEntryBytes: maxEntryBytes}
}

// @sk-task semantic-cache-masked#T1.2: Get loads a cached entry (AC-001)
func (s *ValkeyCacheStore) Get(ctx context.Context, key string) ([]byte, error) {
	if s.client == nil {
		return nil, nil
	}
	val, err := s.client.Do(ctx, s.client.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cache store get: %w", err)
	}
	return []byte(val), nil
}

// @sk-task semantic-cache-masked#T1.2: Set stores an entry with size guard (AC-004)
func (s *ValkeyCacheStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s.client == nil {
		return nil
	}
	if len(value) == 0 {
		return nil
	}
	if s.maxEntryBytes > 0 && int64(len(value)) > s.maxEntryBytes {
		return nil
	}
	resp := s.client.Do(ctx, s.client.B().Set().Key(key).Value(string(value)).ExSeconds(int64(ttl.Seconds())).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("cache store set: %w", err)
	}
	return nil
}
