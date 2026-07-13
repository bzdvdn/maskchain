package profilerepo

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 102-profile-cache#T2.6: Test ProfileCache (AC-001, AC-002, AC-003, AC-004, AC-006, AC-008, AC-009, AC-010)

type mockPGRepo struct {
	shield.ProfileRepository
	mu             sync.RWMutex
	profilesBySlug map[string]*entity.Profile
	profilesByID   map[string]*entity.Profile
	saveFn         func(ctx context.Context, p *entity.Profile) error
	deleteFn       func(ctx context.Context, id value.ProfileID) error
	listFn         func(ctx context.Context, tid value.TenantID) ([]*entity.Profile, error)
}

func newMockPGRepo() *mockPGRepo {
	return &mockPGRepo{
		profilesBySlug: make(map[string]*entity.Profile),
		profilesByID:   make(map[string]*entity.Profile),
	}
}

func (m *mockPGRepo) FindBySlug(ctx context.Context, tenantID value.TenantID, slug value.ProfileSlug) (*entity.Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.profilesBySlug[tenantID.String()+":"+slug.String()]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockPGRepo) FindByID(ctx context.Context, id value.ProfileID) (*entity.Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.profilesByID[id.String()]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockPGRepo) Save(ctx context.Context, p *entity.Profile) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, p)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := p.TenantID().String() + ":" + p.Slug().String()
	m.profilesBySlug[key] = p
	m.profilesByID[p.ID().String()] = p
	return nil
}

func (m *mockPGRepo) Delete(ctx context.Context, id value.ProfileID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.profilesByID, id.String())
	for k, p := range m.profilesBySlug {
		if p.ID().String() == id.String() {
			delete(m.profilesBySlug, k)
			break
		}
	}
	return nil
}

func (m *mockPGRepo) ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Profile, error) {
	if m.listFn != nil {
		return m.listFn(ctx, tenantID)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*entity.Profile
	for _, p := range m.profilesBySlug {
		if p.TenantID().String() == tenantID.String() {
			result = append(result, p)
		}
	}
	return result, nil
}

type mockValkeyCache struct {
	mu              sync.RWMutex
	store           map[string]*profileCacheValue
	getErr          error
	publishedSlugs  []string
}

func newMockValkeyCache() *mockValkeyCache {
	return &mockValkeyCache{store: make(map[string]*profileCacheValue)}
}

func (m *mockValkeyCache) key(tenantID, slug string) string {
	return tenantID + ":" + slug
}

func (m *mockValkeyCache) Get(ctx context.Context, tenantID, slug string) (*profileCacheValue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	v, ok := m.store[m.key(tenantID, slug)]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (m *mockValkeyCache) Set(ctx context.Context, tenantID, slug string, val *profileCacheValue) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[m.key(tenantID, slug)] = val
	return nil
}

func (m *mockValkeyCache) Del(ctx context.Context, tenantID, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, m.key(tenantID, slug))
	return nil
}

func (m *mockValkeyCache) Publish(ctx context.Context, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishedSlugs = append(m.publishedSlugs, slug)
	return nil
}

type mockDictRepo struct {
	dicts map[string]*dictionary.Dictionary
}

func newMockDictRepo() *mockDictRepo {
	return &mockDictRepo{dicts: make(map[string]*dictionary.Dictionary)}
}

func (m *mockDictRepo) FindByProfileSlug(ctx context.Context, slug string) (*dictionary.Dictionary, error) {
	d, ok := m.dicts[slug]
	if !ok {
		return nil, nil
	}
	return d, nil
}

type spyMetrics struct {
	hits          map[string]int
	misses        map[string]int
	stale         map[string]int
	invalidations map[string]int
}

func newSpyMetrics() *spyMetrics {
	return &spyMetrics{
		hits:          make(map[string]int),
		misses:        make(map[string]int),
		stale:         make(map[string]int),
		invalidations: make(map[string]int),
	}
}

func (s *spyMetrics) IncHits(op, level string) {
	s.hits[op+"|"+level]++
}
func (s *spyMetrics) IncMisses(op, level string) {
	s.misses[op+"|"+level]++
}
func (s *spyMetrics) IncStale(op string) {
	s.stale[op]++
}
func (s *spyMetrics) IncInvalidations(op string) {
	s.invalidations[op]++
}

func makeTestProfile(tenantID, slug, name string) *entity.Profile {
	tid, _ := value.NewTenantID(tenantID)
	s, _ := value.NewProfileSlug(slug)
	pid, _ := value.NewProfileID("test-" + slug)
	return entity.NewProfile(pid, s, tid, name,
		entity.WithDictionaries([]*dictionary.Dictionary{
			dictionary.NewDictionary(s, "test-dict", []string{"entry1"}, dictionary.MatchModeExact),
		}),
	)
}

func TestProfileCache_FindBySlug_ValkeyHit(t *testing.T) {
	// AC-003: Valkey hit — no PG call
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewProfileLRUCache(100)
	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	profile := makeTestProfile("t1", "my-profile", "test")
	val := profileToCacheValue(profile, 2)
	vk.store["t1:my-profile"] = val

	cache := NewProfileCache(pg, vk, lru, dictLoader, slog.Default(), nil, metrics, nil)

	tid, _ := value.NewTenantID("t1")
	slug, _ := value.NewProfileSlug("my-profile")
	result, err := cache.FindBySlug(context.Background(), tid, slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected profile, got nil")
	}
	if result.Name() != "test" {
		t.Fatalf("expected name 'test', got %q", result.Name())
	}

	if v := metrics.hits["find_by_slug|valkey"]; v != 1 {
		t.Fatalf("expected 1 valkey hit, got %d", v)
	}
	// PG should not have been populated
	if len(pg.profilesBySlug) != 0 {
		t.Fatal("expected no PG calls")
	}
}

func TestProfileCache_FindBySlug_ReadThrough(t *testing.T) {
	// AC-001: Valkey miss → PG → populate Valkey + LRU
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewProfileLRUCache(100)
	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	profile := makeTestProfile("t1", "my-profile", "test")
	tid, _ := value.NewTenantID("t1")
	slug, _ := value.NewProfileSlug("my-profile")
	pg.profilesBySlug["t1:my-profile"] = profile

	versionCalled := false
	versionFunc := func(ctx context.Context, tenantID, slug string) (int, error) {
		versionCalled = true
		return 1, nil
	}

	cache := NewProfileCache(pg, vk, lru, dictLoader, slog.Default(), versionFunc, metrics, nil)

	result, err := cache.FindBySlug(context.Background(), tid, slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected profile, got nil")
	}
	if result.Name() != "test" {
		t.Fatalf("expected name 'test', got %q", result.Name())
	}

	if !versionCalled {
		t.Fatal("expected versionFunc to be called")
	}
	if v := metrics.misses["find_by_slug|pg"]; v != 1 {
		t.Fatalf("expected 1 PG miss, got %d", v)
	}
	// Valkey populated
	cachedVal, ok := vk.store["t1:my-profile"]
	if !ok {
		t.Fatal("expected Valkey to be populated")
	}
	if cachedVal.Version != 1 {
		t.Fatalf("expected version 1 in valkey, got %d", cachedVal.Version)
	}
	// LRU populated
	lruMeta, ok := lru.Get("t1:my-profile")
	if !ok {
		t.Fatal("expected LRU to be populated")
	}
	if lruMeta.Version != 1 {
		t.Fatalf("expected version 1 in LRU, got %d", lruMeta.Version)
	}
}

func TestProfileCache_Save_WriteThrough(t *testing.T) {
	// AC-002: Save → PG + Valkey + invalidations
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewProfileLRUCache(100)
	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	versionFunc := func(ctx context.Context, tenantID, slug string) (int, error) {
		return 1, nil
	}

	cache := NewProfileCache(pg, vk, lru, dictLoader, slog.Default(), versionFunc, metrics, nil)

	profile := makeTestProfile("t1", "my-profile", "test")
	if err := cache.Save(context.Background(), profile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// PG should have the profile
	if _, exists := pg.profilesBySlug["t1:my-profile"]; !exists {
		t.Fatal("expected profile in PG after Save")
	}
	// Valkey should have the profile
	if _, exists := vk.store["t1:my-profile"]; !exists {
		t.Fatal("expected profile in Valkey after Save")
	}
	if v := metrics.invalidations["save"]; v != 1 {
		t.Fatalf("expected 1 invalidation, got %d", v)
	}
	// LRU should be populated after Save
	lruMeta, ok := lru.Get("t1:my-profile")
	if !ok {
		t.Fatal("expected LRU to be populated after Save")
	}
	if lruMeta.Name != "test" {
		t.Fatalf("expected name 'test' in LRU, got %q", lruMeta.Name)
	}
	if lruMeta.Version != 1 {
		t.Fatalf("expected version 1 in LRU, got %d", lruMeta.Version)
	}
}

func TestProfileCache_FindBySlug_ValkeyError_LRUHit(t *testing.T) {
	// AC-004: Valkey error, LRU has metadata → degraded mode
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	vk.getErr = errors.New("valkey down")
	lru := NewProfileLRUCache(100)

	profile := makeTestProfile("t1", "my-profile", "test")
	meta := ProfileMetadataFromProfile(profile, 2)
	lru.Add("t1:my-profile", meta)

	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	cache := NewProfileCache(pg, vk, lru, dictLoader, slog.Default(), nil, metrics, nil)

	tid, _ := value.NewTenantID("t1")
	slug, _ := value.NewProfileSlug("my-profile")
	result, err := cache.FindBySlug(context.Background(), tid, slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected profile, got nil")
	}
	if result.Name() != "test" {
		t.Fatalf("expected name 'test', got %q", result.Name())
	}

	if v := metrics.hits["find_by_slug|lru"]; v != 1 {
		t.Fatalf("expected 1 LRU hit, got %d", v)
	}
	if v := metrics.stale["find_by_slug"]; v != 1 {
		t.Fatalf("expected 1 stale metric, got %d", v)
	}
}

func TestProfileCache_FindBySlug_ValkeyError_LRUMiss(t *testing.T) {
	// AC-006: Valkey error, LRU empty → full PG read
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	vk.getErr = errors.New("valkey down")
	lru := NewProfileLRUCache(100)

	profile := makeTestProfile("t1", "my-profile", "test")
	tid, _ := value.NewTenantID("t1")
	slug, _ := value.NewProfileSlug("my-profile")
	pg.profilesBySlug["t1:my-profile"] = profile

	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	versionFunc := func(ctx context.Context, tenantID, slug string) (int, error) {
		return 3, nil
	}

	cache := NewProfileCache(pg, vk, lru, dictLoader, slog.Default(), versionFunc, metrics, nil)

	result, err := cache.FindBySlug(context.Background(), tid, slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected profile, got nil")
	}
	if result.Name() != "test" {
		t.Fatalf("expected name 'test', got %q", result.Name())
	}

	if v := metrics.misses["find_by_slug|pg"]; v != 1 {
		t.Fatalf("expected 1 PG miss, got %d", v)
	}
	if v := metrics.stale["find_by_slug"]; v != 1 {
		t.Fatalf("expected 1 stale, got %d", v)
	}
	// LRU populated after PG read
	if _, lruOK := lru.Get("t1:my-profile"); !lruOK {
		t.Fatal("expected LRU to be populated after PG read")
	}
}

func TestProfileCache_Delete(t *testing.T) {
	// AC-009: Delete → PG + Valkey + LRU eviction
	pg := newMockPGRepo()
	vk := newMockValkeyCache()
	lru := NewProfileLRUCache(100)

	profile := makeTestProfile("t1", "my-profile", "test")

	pg.profilesByID[profile.ID().String()] = profile
	pg.profilesBySlug["t1:my-profile"] = profile
	vk.store["t1:my-profile"] = profileToCacheValue(profile, 1)
	lru.Add("t1:my-profile", ProfileMetadataFromProfile(profile, 1))

	dictLoader := NewDictLoader(newMockDictRepo().FindByProfileSlug)
	metrics := newSpyMetrics()

	cache := NewProfileCache(pg, vk, lru, dictLoader, slog.Default(), nil, metrics, nil)

	if err := cache.Delete(context.Background(), profile.ID()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// PG: profile deleted
	if _, pgExists := pg.profilesByID[profile.ID().String()]; pgExists {
		t.Fatal("expected profile to be deleted from PG")
	}
	// Valkey: key deleted
	if _, vkExists := vk.store["t1:my-profile"]; vkExists {
		t.Fatal("expected profile to be deleted from Valkey")
	}
	// LRU: evicted
	if _, lruOK := lru.Get("t1:my-profile"); lruOK {
		t.Fatal("expected profile to be evicted from LRU")
	}
	if v := metrics.invalidations["delete"]; v != 1 {
		t.Fatalf("expected 1 delete invalidation, got %d", v)
	}
}

func TestProfileCache_PromMetrics(t *testing.T) {
	// AC-008: Metrics export — verify counters are wired correctly
	reg := prometheus.NewRegistry()
	hits := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_hits"}, []string{"operation", "level"})
	misses := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_misses"}, []string{"operation", "level"})
	stale := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stale"}, []string{"operation"})
	inv := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_inv"}, []string{"operation"})
	reg.MustRegister(hits, misses, stale, inv)

	m := NewPromCacheMetrics(hits, misses, stale, inv)
	m.IncHits("find_by_slug", "valkey")
	m.IncMisses("find_by_slug", "pg")
	m.IncStale("find_by_slug")
	m.IncInvalidations("save")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(families) != 4 {
		t.Fatalf("expected 4 metric families, got %d", len(families))
	}
}
