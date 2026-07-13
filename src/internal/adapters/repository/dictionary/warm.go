package dictionaryrepo

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 102-profile-cache#T2.4: Implement DictionaryCacheWarmer (RQ-013)
// @sk-task tenant-profile-sync#T4.1: Rename ProfileCacheWarmer → DictionaryCacheWarmer (AC-006)
type DictionaryCacheWarmer struct {
	pgRepo      shield.ProfileRepository
	valkeyRepo  profileValkeyCache
	lru         *DictionaryLRUCache
	logger      *slog.Logger
	versionFunc DictionaryVersionFunc
	concurrency int
}

func NewDictionaryCacheWarmer(
	pgRepo shield.ProfileRepository,
	valkeyRepo profileValkeyCache,
	lru *DictionaryLRUCache,
	logger *slog.Logger,
	versionFunc DictionaryVersionFunc,
	concurrency int,
) *DictionaryCacheWarmer {
	return &DictionaryCacheWarmer{
		pgRepo:      pgRepo,
		valkeyRepo:  valkeyRepo,
		lru:         lru,
		logger:      logger,
		versionFunc: versionFunc,
		concurrency: concurrency,
	}
}

// WarmTenant warms the cache for all profiles of a given tenant.
func (w *DictionaryCacheWarmer) WarmTenant(ctx context.Context, tenantID value.TenantID) {
	profiles, err := w.pgRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		w.logger.Warn("cache warmer: list profiles failed", "tenant", tenantID.String(), "error", err)
		return
	}

	sem := make(chan struct{}, w.concurrency)
	var wg sync.WaitGroup

	for _, p := range profiles {
		ref := dictRef{slug: p.Slug().String(), tenantID: p.TenantID().String()}

		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(r dictRef) {
			defer func() {
				<-sem
				wg.Done()
			}()
			if err := w.warmOne(ctx, &r); err != nil {
				w.logger.Warn("cache warmer: warm failed", "slug", r.slug, "error", err)
			}
		}(ref)
	}

	wg.Wait()
}

type dictRef struct {
	slug     string
	tenantID string
}

func (w *DictionaryCacheWarmer) warmOne(ctx context.Context, ref *dictRef) error {
	val, err := w.valkeyRepo.Get(ctx, ref.tenantID, ref.slug)
	if err == nil && val != nil {
		profile, convErr := cacheValueToDict(val)
		if convErr != nil {
			return convErr
		}
		w.lru.Add(metadataKey(ref.tenantID, ref.slug), DictionaryMetadataFromProfile(profile, val.Version))
		return nil
	}

	slugVal, err := value.NewProfileSlug(ref.slug)
	if err != nil {
		return err
	}
	tid, err := value.NewTenantID(ref.tenantID)
	if err != nil {
		return err
	}

	profile, err := w.pgRepo.FindBySlug(ctx, tid, slugVal)
	if err != nil {
		return err
	}
	if profile == nil {
		return nil
	}

	version := 0
	if w.versionFunc != nil {
		if v, verr := w.versionFunc(ctx, ref.tenantID, ref.slug); verr == nil {
			version = v
		}
	}

	val = dictToCacheValue(profile, version)
	if err := w.valkeyRepo.Set(ctx, ref.tenantID, ref.slug, val); err != nil {
		w.logger.Warn("cache warmer: valkey set failed", "slug", ref.slug, "error", err)
	}
	w.lru.Add(metadataKey(ref.tenantID, ref.slug), DictionaryMetadataFromProfile(profile, version))
	return nil
}
