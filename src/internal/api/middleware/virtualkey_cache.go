package middleware

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// virtualKeyLister is the minimal surface the cache needs to refresh itself.
// *postgres.PostgresVirtualKeyRepository and test fakes satisfy it.
type virtualKeyLister interface {
	List(ctx context.Context) ([]*virtualkey.VirtualKey, error)
}

// DefaultVirtualKeyCacheRefresh is the fallback refresh interval used when the
// bootstrap passes a non-positive interval.
const DefaultVirtualKeyCacheRefresh = 30 * time.Second

// @sk-task 403-key-at-rest-encryption#T3.1: VirtualKeyCache is an in-process auth index (AC-005)
//
// VirtualKeyCache keeps virtual keys indexed by SHA-256 key hash so request-time
// authentication is 0-DB after warm-up. The DB remains authoritative; the cache
// is refreshed periodically and invalidated on admin key mutations.
type VirtualKeyCache struct {
	mu     sync.RWMutex
	byHash map[string]*virtualkey.VirtualKey
	repo   virtualKeyLister
	logger *slog.Logger
}

// @sk-task 403-key-at-rest-encryption#T3.1: NewVirtualKeyCache builds an empty cache (AC-005)
func NewVirtualKeyCache(repo virtualKeyLister, logger *slog.Logger) *VirtualKeyCache {
	return &VirtualKeyCache{
		byHash: make(map[string]*virtualkey.VirtualKey),
		repo:   repo,
		logger: logger,
	}
}

// @sk-task 403-key-at-rest-encryption#T3.1: Start begins periodic refresh until ctx is cancelled (AC-005)
//
// Start performs an initial refresh, then refreshes every interval. A
// non-positive interval falls back to DefaultVirtualKeyCacheRefresh.
func (c *VirtualKeyCache) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultVirtualKeyCacheRefresh
	}
	if err := c.Refresh(ctx); err != nil {
		c.logger.Warn("virtual key cache initial refresh failed", slog.String("error", err.Error()))
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Refresh(ctx); err != nil {
				c.logger.Warn("virtual key cache refresh failed", slog.String("error", err.Error()))
				continue
			}
			c.logger.Debug("virtual key cache refreshed", slog.Int("keys", c.Len()))
		}
	}
}

// @sk-task 403-key-at-rest-encryption#T3.1: Refresh reloads the index from the repository (AC-005)
func (c *VirtualKeyCache) Refresh(ctx context.Context) error {
	keys, err := c.repo.List(ctx)
	if err != nil {
		return err
	}
	next := make(map[string]*virtualkey.VirtualKey, len(keys))
	for _, k := range keys {
		next[k.KeyHash] = k
	}
	c.mu.Lock()
	c.byHash = next
	c.mu.Unlock()
	return nil
}

// @sk-task 403-key-at-rest-encryption#T3.1: Get returns the cached key for a hash (AC-005)
func (c *VirtualKeyCache) Get(hash string) (*virtualkey.VirtualKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	vk, ok := c.byHash[hash]
	return vk, ok
}

// @sk-task 403-key-at-rest-encryption#T3.1: Set upserts a key into the index (AC-005)
func (c *VirtualKeyCache) Set(hash string, vk *virtualkey.VirtualKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byHash[hash] = vk
}

// @sk-task 403-key-at-rest-encryption#T3.1: InvalidateKey removes a key by id (AC-005, AC-007)
func (c *VirtualKeyCache) InvalidateKey(keyID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for hash, vk := range c.byHash {
		if vk.ID == keyID {
			delete(c.byHash, hash)
		}
	}
}

// @sk-task 403-key-at-rest-encryption#T3.1: InvalidateTenant removes all keys of a tenant (AC-005)
func (c *VirtualKeyCache) InvalidateTenant(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for hash, vk := range c.byHash {
		if vk.TenantID == tenantID {
			delete(c.byHash, hash)
		}
	}
}

// @sk-task 403-key-at-rest-encryption#T3.1: Len returns the number of cached keys (AC-005)
func (c *VirtualKeyCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.byHash)
}
