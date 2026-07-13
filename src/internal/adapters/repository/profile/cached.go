package profilerepo

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// ProfileVersionFunc retrieves the current version of a profile from PG.
type ProfileVersionFunc func(ctx context.Context, tenantID, slug string) (int, error)

type profileValkeyCache interface {
	Get(ctx context.Context, tenantID, slug string) (*profileCacheValue, error)
	Set(ctx context.Context, tenantID, slug string, val *profileCacheValue) error
	Del(ctx context.Context, tenantID, slug string) error
	Publish(ctx context.Context, slug string) error
}

var _ profileValkeyCache = (*ProfileValkeyRepo)(nil)

// InvalidationTracker records slugs invalidated via PubSub for LRU skip-on-read.
type InvalidationTracker struct {
	mu  sync.RWMutex
	set map[string]struct{}
}

func NewInvalidationTracker() *InvalidationTracker {
	return &InvalidationTracker{set: make(map[string]struct{})}
}

func (t *InvalidationTracker) Add(slug string) {
	t.mu.Lock()
	t.set[slug] = struct{}{}
	t.mu.Unlock()
}

// CheckAndClear returns true if slug was invalidated and removes it.
func (t *InvalidationTracker) CheckAndClear(slug string) bool {
	t.mu.Lock()
	_, ok := t.set[slug]
	if ok {
		delete(t.set, slug)
	}
	t.mu.Unlock()
	return ok
}

// @sk-task 102-profile-cache#T2.2: Implement ProfileCache (RQ-001, RQ-003, RQ-004, RQ-005, RQ-007, RQ-009, RQ-010, RQ-011)
// @sk-task 102-profile-cache#T3.1: Add PubSub publish to Save/Delete (AC-005, AC-007)
type ProfileCache struct {
	pgRepo            shield.ProfileRepository
	valkeyRepo        profileValkeyCache
	lru               *ProfileLRUCache
	dictRepo          *DictLoader
	logger            *slog.Logger
	versionFunc       ProfileVersionFunc
	metrics           cacheMetrics
	invalidated       *InvalidationTracker
}

type cacheMetrics interface {
	IncHits(operation, level string)
	IncMisses(operation, level string)
	IncStale(operation string)
	IncInvalidations(operation string)
}

func NewProfileCache(
	pgRepo shield.ProfileRepository,
	valkeyRepo profileValkeyCache,
	lru *ProfileLRUCache,
	dictLoader *DictLoader,
	logger *slog.Logger,
	versionFunc ProfileVersionFunc,
	metrics cacheMetrics,
	invalidated *InvalidationTracker,
) *ProfileCache {
	return &ProfileCache{
		pgRepo:      pgRepo,
		valkeyRepo:  valkeyRepo,
		lru:         lru,
		dictRepo:    dictLoader,
		logger:      logger,
		versionFunc: versionFunc,
		metrics:     metrics,
		invalidated: invalidated,
	}
}

var _ shield.ProfileRepository = (*ProfileCache)(nil)

// DictLoader is a thin wrapper that loads dictionary.Dictionary by profile slug.
type DictLoader struct {
	loadFn func(ctx context.Context, slug string) (*dictionary.Dictionary, error)
}

func NewDictLoader(loadFn func(ctx context.Context, slug string) (*dictionary.Dictionary, error)) *DictLoader {
	return &DictLoader{loadFn: loadFn}
}

func (d *DictLoader) FindByProfileSlug(ctx context.Context, slug string) (*dictionary.Dictionary, error) {
	return d.loadFn(ctx, slug)
}

func (c *ProfileCache) Save(ctx context.Context, profile *entity.Profile) error {
	if err := c.pgRepo.Save(ctx, profile); err != nil {
		return fmt.Errorf("pg save: %w", err)
	}

	version := c.resolveVersion(ctx, profile.TenantID().String(), profile.Slug().String())

	val := profileToCacheValue(profile, version)
	slugStr := profile.Slug().String()
	if err := c.valkeyRepo.Set(ctx, profile.TenantID().String(), slugStr, val); err != nil {
		c.logger.Warn("valkey set failed after save", "slug", slugStr, "error", err)
		c.metrics.IncStale("save")
	} else {
		c.metrics.IncInvalidations("save")
	}
	if pubErr := c.valkeyRepo.Publish(ctx, slugStr); pubErr != nil {
		c.logger.Warn("pubsub publish failed on save", "slug", slugStr, "error", pubErr)
	}

	c.lru.Add(metadataKey(profile.TenantID().String(), slugStr), ProfileMetadataFromProfile(profile, version))
	return nil
}

func (c *ProfileCache) FindBySlug(ctx context.Context, tenantID value.TenantID, slug value.ProfileSlug) (*entity.Profile, error) {
	tenantStr := tenantID.String()
	slugStr := slug.String()

	val, err := c.valkeyRepo.Get(ctx, tenantStr, slugStr)
	if err == nil && val != nil {
		c.metrics.IncHits("find_by_slug", "valkey")
		return cacheValueToProfile(val)
	}
	if err != nil {
		c.logger.Warn("valkey get failed on FindBySlug", "slug", slugStr, "error", err)
		c.metrics.IncStale("find_by_slug")
	}

	if c.invalidated != nil && c.invalidated.CheckAndClear(slugStr) {
		c.logger.Debug("lru skip: slug invalidated via pubsub", "slug", slugStr)
	} else if meta, metaOK := c.lru.Get(metadataKey(tenantStr, slugStr)); err != nil && metaOK {
		c.metrics.IncHits("find_by_slug", "lru")
		profile, loadErr := c.assembleDegraded(ctx, tenantStr, slugStr, meta)
		if loadErr == nil {
			return profile, nil
		}
		c.logger.Warn("degraded assembly failed", "slug", slugStr, "error", loadErr)
	}

	profile, err := c.pgRepo.FindBySlug(ctx, tenantID, slug)
	if err != nil {
		return nil, fmt.Errorf("pg FindBySlug: %w", err)
	}
	if profile == nil {
		return nil, nil
	}
	c.metrics.IncMisses("find_by_slug", "pg")

	version := c.resolveVersion(ctx, tenantStr, slugStr)
	val = profileToCacheValue(profile, version)
	if setErr := c.valkeyRepo.Set(ctx, tenantStr, slugStr, val); setErr != nil {
		c.logger.Warn("valkey set failed on miss", "slug", slugStr, "error", setErr)
	}
	c.lru.Add(metadataKey(tenantStr, slugStr), ProfileMetadataFromProfile(profile, version))
	return profile, nil
}

func (c *ProfileCache) FindByID(ctx context.Context, id value.ProfileID) (*entity.Profile, error) {
	return c.pgRepo.FindByID(ctx, id)
}

func (c *ProfileCache) ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Profile, error) {
	return c.pgRepo.ListByTenant(ctx, tenantID)
}

func (c *ProfileCache) Delete(ctx context.Context, id value.ProfileID) error {
	profile, err := c.pgRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("pg find for delete: %w", err)
	}
	if profile == nil {
		return nil
	}

	if err := c.pgRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("pg delete: %w", err)
	}

	tenantStr := profile.TenantID().String()
	slugStr := profile.Slug().String()

	if delErr := c.valkeyRepo.Del(ctx, tenantStr, slugStr); delErr != nil {
		c.logger.Warn("valkey del failed on delete", "slug", slugStr, "error", delErr)
	}
	if pubErr := c.valkeyRepo.Publish(ctx, slugStr); pubErr != nil {
		c.logger.Warn("pubsub publish failed on delete", "slug", slugStr, "error", pubErr)
	}
	c.lru.Remove(metadataKey(tenantStr, slugStr))
	c.metrics.IncInvalidations("delete")
	return nil
}

func (c *ProfileCache) assembleDegraded(ctx context.Context, tenantID, slug string, meta *ProfileMetadata) (*entity.Profile, error) {
	pid, err := value.NewProfileID(meta.ID)
	if err != nil {
		return nil, err
	}
	slugVal, err := value.NewProfileSlug(meta.Slug)
	if err != nil {
		return nil, err
	}
	tid, err := value.NewTenantID(meta.TenantID)
	if err != nil {
		return nil, err
	}

	opts := []entity.ProfileOption{
		entity.WithEnabled(meta.Enabled),
		entity.WithDetectors(meta.Detectors),
		entity.WithPreprocessors(meta.Preprocessors),
	}
	if meta.Description != nil {
		opts = append(opts, entity.WithDescription(*meta.Description))
	}

	dict, dictErr := c.dictRepo.FindByProfileSlug(ctx, slug)
	if dictErr == nil && dict != nil {
		opts = append(opts, entity.WithDictionaries([]*dictionary.Dictionary{dict}))
	} else if dictErr != nil {
		c.logger.Warn("degraded: dict load failed", "slug", slug, "error", dictErr)
	}

	return entity.NewProfile(pid, slugVal, tid, meta.Name, opts...), nil
}

func (c *ProfileCache) resolveVersion(ctx context.Context, tenantID, slug string) int {
	if c.versionFunc == nil {
		return 0
	}
	v, err := c.versionFunc(ctx, tenantID, slug)
	if err != nil {
		c.logger.Warn("version resolve failed", "slug", slug, "error", err)
		return 0
	}
	return v
}
