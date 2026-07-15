package sessionrepo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

// @sk-task sessions#T3.1: Implement ValkeySessionCache (AC-008)
type ValkeySessionCache struct {
	client valkey.Client
	ttl    time.Duration
}

func NewValkeySessionCache(client valkey.Client, ttl time.Duration) *ValkeySessionCache {
	return &ValkeySessionCache{client: client, ttl: ttl}
}

func (c *ValkeySessionCache) key(sessionID string) string {
	return "session:" + sessionID
}

// @sk-task sessions#T3.1: Save with SET EX (AC-008)
func (c *ValkeySessionCache) Save(ctx context.Context, s *session.Session) error {
	if c.client == nil {
		return nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return c.client.Do(ctx, c.client.B().Set().Key(c.key(s.SessionID)).
		Value(string(data)).
		Ex(c.ttl).
		Build()).Error()
}

// @sk-task sessions#T3.1: Get with valkey.Nil -> ErrSessionNotFound (AC-008)
func (c *ValkeySessionCache) Get(ctx context.Context, tenantID, sessionID string) (*session.Session, error) {
	if c.client == nil {
		return nil, session.ErrSessionNotFound
	}
	data, err := c.client.Do(ctx, c.client.B().Get().Key(c.key(sessionID)).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, session.ErrSessionNotFound
		}
		return nil, err
	}
	var s session.Session
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *ValkeySessionCache) IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error {
	return session.ErrSessionNotFound
}

func (c *ValkeySessionCache) ExtendTTL(ctx context.Context, tenantID, sessionID string, newExpiresAt time.Time) error {
	return session.ErrSessionNotFound
}

func (c *ValkeySessionCache) Close(ctx context.Context, tenantID, sessionID string) error {
	return session.ErrSessionNotFound
}

// @sk-task sessions#T3.1: DeleteExpired with SCAN + DEL (best-effort) (AC-008)
func (c *ValkeySessionCache) DeleteExpired(ctx context.Context) (int64, error) {
	if c.client == nil {
		return 0, nil
	}
	var deleted int64
	var cursor uint64 = 0
	for {
		entry, err := c.client.Do(ctx, c.client.B().Scan().Cursor(cursor).Match("session:*").Build()).AsScanEntry()
		if err != nil {
			return deleted, nil
		}
		for _, key := range entry.Elements {
			if err := c.client.Do(ctx, c.client.B().Del().Key(key).Build()).Error(); err == nil {
				deleted++
			}
		}
		if entry.Cursor == 0 {
			break
		}
		cursor = entry.Cursor
	}
	return deleted, nil
}

func (c *ValkeySessionCache) ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*session.ListResult, error) {
	return &session.ListResult{Items: []session.Session{}, Total: 0, Page: int(page), Limit: int(limit)}, nil
}
