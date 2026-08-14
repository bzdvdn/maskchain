package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// countingLister is a virtualKeyLister that records how many times List is called.
type countingLister struct {
	mu   sync.Mutex
	keys []*virtualkey.VirtualKey
	n    int
}

func (c *countingLister) List(_ context.Context) ([]*virtualkey.VirtualKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	out := make([]*virtualkey.VirtualKey, len(c.keys))
	copy(out, c.keys)
	return out, nil
}

func (c *countingLister) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// countingKeyRepo counts FindByKeyHash calls so tests can prove the cache
// absorbs request-time lookups.
type countingKeyRepo struct {
	repo *fakeVirtualKeyRepo
	mu   sync.Mutex
	n    int
}

func newCountingKeyRepo(keys ...*virtualkey.VirtualKey) *countingKeyRepo {
	return &countingKeyRepo{repo: &fakeVirtualKeyRepo{keys: keys}}
}

func (c *countingKeyRepo) FindByKeyHash(ctx context.Context, hash string) (*virtualkey.VirtualKey, error) {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return c.repo.FindByKeyHash(ctx, hash)
}

func (c *countingKeyRepo) GetById(ctx context.Context, id string) (*virtualkey.VirtualKey, error) {
	return c.repo.GetById(ctx, id)
}

func (c *countingKeyRepo) ListByTenant(ctx context.Context, tenantID string) ([]*virtualkey.VirtualKey, error) {
	return c.repo.ListByTenant(ctx, tenantID)
}

func (c *countingKeyRepo) List(ctx context.Context) ([]*virtualkey.VirtualKey, error) {
	return c.repo.List(ctx)
}

func (c *countingKeyRepo) Create(ctx context.Context, k *virtualkey.VirtualKey) error {
	return c.repo.Create(ctx, k)
}

func (c *countingKeyRepo) Update(ctx context.Context, k *virtualkey.VirtualKey) error {
	return c.repo.Update(ctx, k)
}

func (c *countingKeyRepo) Delete(ctx context.Context, id string) error {
	return c.repo.Delete(ctx, id)
}

func (c *countingKeyRepo) BackfillFromLegacy(ctx context.Context, m map[string][]string) (int, error) {
	return c.repo.BackfillFromLegacy(ctx, m)
}

func (c *countingKeyRepo) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyCacheRefreshIndexes (AC-005)
func TestVirtualKeyCacheRefreshIndexes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := NewVirtualKeyCache(&countingLister{}, logger)

	repo := &countingLister{keys: []*virtualkey.VirtualKey{fakeKey("h1", "acme", nil, nil)}}
	cache.repo = repo

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if cache.Len() != 1 {
		t.Fatalf("expected 1 cached key, got %d", cache.Len())
	}
	vk, ok := cache.Get("h1")
	if !ok || vk.TenantID != "acme" {
		t.Errorf("expected cached key h1 for acme, got %v", vk)
	}
	if repo.calls() != 1 {
		t.Errorf("expected exactly 1 List call, got %d", repo.calls())
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyCacheInvalidateKey (AC-005, AC-007)
func TestVirtualKeyCacheInvalidateKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{
		fakeKey("h1", "acme", nil, nil),
		fakeKey("h2", "acme", nil, nil),
		fakeKey("h3", "beta", nil, nil),
	}}
	cache := NewVirtualKeyCache(repo, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if cache.Len() != 3 {
		t.Fatalf("expected 3 cached keys, got %d", cache.Len())
	}

	cache.InvalidateKey("id-h1")
	if _, ok := cache.Get("h1"); ok {
		t.Error("expected h1 to be invalidated")
	}
	if _, ok := cache.Get("h2"); !ok {
		t.Error("expected h2 to remain cached")
	}
	if cache.Len() != 2 {
		t.Errorf("expected 2 cached keys after invalidation, got %d", cache.Len())
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyCacheInvalidateTenant (AC-005)
func TestVirtualKeyCacheInvalidateTenant(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{
		fakeKey("h1", "acme", nil, nil),
		fakeKey("h2", "acme", nil, nil),
		fakeKey("h3", "beta", nil, nil),
	}}
	cache := NewVirtualKeyCache(repo, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	cache.InvalidateTenant("acme")
	if _, ok := cache.Get("h1"); ok {
		t.Error("expected h1 to be invalidated")
	}
	if _, ok := cache.Get("h2"); ok {
		t.Error("expected h2 to be invalidated")
	}
	if _, ok := cache.Get("h3"); !ok {
		t.Error("expected h3 to remain cached")
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyAuthServesFromCacheAfterWarmup (AC-005, AC-006)
func TestVirtualKeyAuthServesFromCacheAfterWarmup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ta, tenants := setupAuthTest()
	key := fakeKey("", "alpha", nil, nil)
	key.KeyHash = virtualkey.KeyHash("sk-abc")

	repo := newCountingKeyRepo(key)
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := NewVirtualKeyCache(&fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{key}}, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("warmup refresh: %v", err)
	}

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(VirtualKeyAuth(repo, NewTenantProvider(tenants), cache))
	engine.GET("/api/v1/profiles", func(c *gin.Context) {
		vk, ok := VirtualKeyFromContext(c)
		if !ok || vk == nil || vk.TenantID != "alpha" {
			t.Errorf("expected virtual key in context, got %v", vk)
		}
		gotT := ta
		if ctxTenant, ok := TenantFromContext(c); ok {
			gotT = ctxTenant
		}
		if gotT.Slug().String() != "alpha" {
			t.Errorf("expected tenant alpha, got %s", gotT.Slug().String())
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer sk-abc")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if repo.calls() != 0 {
		t.Errorf("expected 0 repo FindByKeyHash calls with warm cache, got %d", repo.calls())
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyAuthFallsBackToRepoOnMiss (AC-005, AC-006)
func TestVirtualKeyAuthFallsBackToRepoOnMiss(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, tenants := setupAuthTest()
	key := fakeKey("", "alpha", nil, nil)
	key.KeyHash = virtualkey.KeyHash("bb-key")

	repo := newCountingKeyRepo(key)
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := NewVirtualKeyCache(&fakeVirtualKeyRepo{}, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(VirtualKeyAuth(repo, NewTenantProvider(tenants), cache))
	engine.GET("/api/v1/profiles", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer bb-key")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if repo.calls() != 1 {
		t.Errorf("expected 1 repo call on cache miss, got %d", repo.calls())
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyCacheStartRefreshesPeriodically (AC-005, AC-006)
func TestVirtualKeyCacheStartRefreshesPeriodically(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	repo := &countingLister{keys: []*virtualkey.VirtualKey{fakeKey("h1", "acme", nil, nil)}}
	cache := NewVirtualKeyCache(repo, logger)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		cache.Start(ctx, 5*time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if repo.calls() >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	if repo.calls() < 3 {
		t.Errorf("expected at least 3 refreshes (initial + ticks), got %d", repo.calls())
	}
	if cache.Len() != 1 {
		t.Errorf("expected 1 cached key, got %d", cache.Len())
	}
}
