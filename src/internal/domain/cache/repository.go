package cache

import (
	"context"
	"time"
)

// @sk-task semantic-cache-masked#T1.1: Embedder port (AC-001)
//
// Embedder maps natural language text to a vector embedding. Implementations
// may call an external OpenAI-compatible API or compute offline embeddings.
type Embedder interface {
	// Embed returns the embedding vector for text. Embeds masked text only.
	Embed(ctx context.Context, text string) ([]float32, error)
}

// @sk-task semantic-cache-masked#T1.1: Store port (AC-001, AC-004)
//
// Store persists and fetches serialized cache entries keyed by CacheKey.
// A miss is reported as (nil, nil); a stored entry is returned as bytes.
type Store interface {
	// Get returns the stored entry bytes, or (nil, nil) on miss.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores value for the given TTL.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}
