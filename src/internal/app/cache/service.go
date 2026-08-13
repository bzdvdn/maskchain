package cacheapp

import (
	"context"
	"time"

	domaincache "github.com/bzdvdn/maskchain/src/internal/domain/cache"
)

// @sk-task semantic-cache-masked#T2.1: SemanticCacheService lookup + store (AC-001, AC-002)
//
// SemanticCacheService embeds masked text, derives the tenant-scoped cache key
// and serves/stores masked responses. A hit returns the masked payload; a miss
// returns nil so the caller proceeds to the provider.
type SemanticCacheService struct {
	embedder  domaincache.Embedder
	store     domaincache.Store
	guard     WriteGuard
	ttl       time.Duration
	threshold float64
}

func NewSemanticCacheService(embedder domaincache.Embedder, store domaincache.Store, ttl time.Duration, threshold float64) *SemanticCacheService {
	return &SemanticCacheService{
		embedder:  embedder,
		store:     store,
		ttl:       ttl,
		threshold: threshold,
	}
}

// WithGuard attaches a budget-aware write guard.
func (s *SemanticCacheService) WithGuard(g WriteGuard) *SemanticCacheService {
	s.guard = g
	return s
}

// Embed returns the embedding of the masked text.
func (s *SemanticCacheService) Embed(ctx context.Context, maskedText string) ([]float32, error) {
	return s.embedder.Embed(ctx, maskedText)
}

// @sk-task semantic-cache-masked#T2.1: Lookup returns the cached masked payload or nil on miss (AC-001)
func (s *SemanticCacheService) Lookup(ctx context.Context, tenantID, maskedText string) ([]byte, error) {
	emb, err := s.embedder.Embed(ctx, maskedText)
	if err != nil {
		return nil, err
	}
	key, err := domaincache.CacheKey(tenantID, emb)
	if err != nil {
		return nil, err
	}
	raw, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	entry, err := domaincache.UnmarshalCacheEntry(raw)
	if err != nil {
		return nil, nil
	}
	if entry.Expired(time.Now().UTC()) {
		return nil, nil
	}
	return entry.Payload, nil
}

// @sk-task semantic-cache-masked#T2.1: Store writes the masked response with tenant TTL (AC-002)
// @sk-task semantic-cache-masked#T3.2: Store is blocked by the budget guard (AC-008)
func (s *SemanticCacheService) Store(ctx context.Context, tenantID, maskedText string, payload []byte) (blocked bool, err error) {
	if s.guard != nil && !s.guard.CanWrite(ctx, tenantID) {
		return true, nil
	}
	if len(payload) == 0 {
		return false, nil
	}
	emb, err := s.embedder.Embed(ctx, maskedText)
	if err != nil {
		return false, err
	}
	key, err := domaincache.CacheKey(tenantID, emb)
	if err != nil {
		return false, err
	}
	entry := domaincache.NewCacheEntry(payload, s.ttl)
	data, err := entry.Marshal()
	if err != nil {
		return false, err
	}
	return false, s.store.Set(ctx, key, data, s.ttl)
}
