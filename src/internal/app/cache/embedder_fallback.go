package cacheapp

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"

	domaincache "github.com/bzdvdn/maskchain/src/internal/domain/cache"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// @sk-task semantic-cache-masked#T3.1: SelfContainedEmbedder offline fallback (AC-006)
//
// SelfContainedEmbedder produces a deterministic embedding from masked text
// without any network call. It is used either as the configured source or as a
// fallback when the external embedding endpoint is unavailable.
type SelfContainedEmbedder struct{}

func NewSelfContainedEmbedder() *SelfContainedEmbedder {
	return &SelfContainedEmbedder{}
}

// @sk-task semantic-cache-masked#T3.1: Embed hashes masked text deterministically (AC-006)
func (e *SelfContainedEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("embedder: empty text")
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(text))
	sum := h.Sum64()
	return []float32{
		float32(int64(sum)) / float32(math.MaxInt64),
		float32(len(text)),
	}, nil
}

// @sk-task semantic-cache-masked#T3.1: FallbackEmbedder degrades to offline (AC-006)
//
// FallbackEmbedder tries the primary (external) embedder and transparently
// falls back to an offline embedder on any error, counting a cache error so
// operators observe the degradation. Clients always get a 2xx path.
type FallbackEmbedder struct {
	primary  domaincache.Embedder
	fallback domaincache.Embedder
}

func NewFallbackEmbedder(primary, fallback domaincache.Embedder) *FallbackEmbedder {
	if fallback == nil {
		fallback = NewSelfContainedEmbedder()
	}
	return &FallbackEmbedder{primary: primary, fallback: fallback}
}

// @sk-task semantic-cache-masked#T3.1: Embed falls back on primary failure (AC-006)
func (f *FallbackEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embedding, err := f.primary.Embed(ctx, text)
	if err == nil {
		return embedding, nil
	}
	metrics.CacheErrorsTotal.WithLabelValues(TenantFrom(ctx)).Inc()
	return f.fallback.Embed(ctx, text)
}
