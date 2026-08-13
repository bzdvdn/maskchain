package bootstrap

import (
	"log/slog"
	"time"

	"github.com/valkey-io/valkey-go"

	cacherepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/cache"
	cacheapp "github.com/bzdvdn/maskchain/src/internal/app/cache"
	cacheBudget "github.com/bzdvdn/maskchain/src/internal/domain/budget"
	"github.com/bzdvdn/maskchain/src/internal/domain/cache"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task semantic-cache-masked#T3.2: budget-aware write guard (AC-008)
//
// NewBudgetWriteGuard builds a cache write guard backed by budget repo/counter.
// Returns nil when the budget stores are unavailable (guard then never blocks).
func NewBudgetWriteGuard(repo cacheBudget.BudgetRepository, counter cacheBudget.SpendCounter, guardPercent float64) *cacheapp.BudgetWriteGuard {
	if repo == nil || counter == nil {
		return nil
	}
	return cacheapp.NewBudgetWriteGuard(repo, counter, guardPercent)
}

// NewSemanticCacheService wires the semantic cache service from config and a
// Valkey client. Returns nil when caching is disabled or Valkey is absent.
func NewSemanticCacheService(cfg *config.CacheConfig, vkClient valkey.Client, log *slog.Logger) *cacheapp.SemanticCacheService {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if vkClient == nil {
		log.Warn("semantic cache enabled but Valkey unavailable — cache disabled")
		return nil
	}
	if cfg.Embedding == nil {
		log.Warn("semantic cache enabled but embedding config missing — cache disabled")
		return nil
	}
	ttl := time.Duration(cfg.TTLSec) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	timeout := time.Duration(cfg.Embedding.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	store := cacherepo.NewValkeyCacheStore(vkClient, cfg.MaxEntryBytes)
	var embedder cache.Embedder
	switch cfg.Embedding.Source {
	case "self-contained":
		embedder = cacheapp.NewSelfContainedEmbedder()
	default:
		external := cacheapp.NewExternalEmbedder(cfg.Embedding.ExternalURL, timeout)
		embedder = cacheapp.NewFallbackEmbedder(external, cacheapp.NewSelfContainedEmbedder())
	}
	return cacheapp.NewSemanticCacheService(embedder, store, ttl, cfg.SimilarityThreshold)
}
