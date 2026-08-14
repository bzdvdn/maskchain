package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"

	cacheapp "github.com/bzdvdn/maskchain/src/internal/app/cache"
	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/metrics"
)

// --- fakes ---

type memoryStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (s *memoryStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		return nil, nil
	}
	v, ok := s.data[key]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (s *memoryStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	s.data[key] = value
	return nil
}

func (s *memoryStore) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.data))
	for k := range s.data {
		out = append(out, k)
	}
	return out
}

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	h := uint32(2166136261)
	for i := 0; i < len(text); i++ {
		h ^= uint32(text[i])
		h *= 16777619
	}
	return []float32{float32(int32(h)) / float32(math.MaxInt32), float32(len(text))}, nil
}

func newCacheTestEngineExt(enabled bool, calls *int, svc *cacheapp.SemanticCacheService, store *memoryStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	cfg := &config.CacheConfig{Enabled: enabled, TTLSec: 60}
	mw := NewSemanticCacheMiddleware(svc, cfg, slog.Default())

	slug, _ := value.NewTenantSlug("alpha")
	tenant := entity.NewTenant(slug, "Alpha", "X-Mask-Auth", entity.WithTenantDictionaries(nil))

	engine.Use(func(c *gin.Context) {
		c.Set(tenantKey, tenant)
		c.Next()
	})
	engine.Use(mw.Handler())
	engine.POST("/api/v1/chat/completions", func(c *gin.Context) {
		*calls++
		c.Data(http.StatusOK, "application/json", []byte(`{"choices":[{"message":{"content":"[MASK_ab.0] reply"}}]}`))
	})
	return engine
}

func newCacheTestEngine(enabled bool, calls *int) (*gin.Engine, *memoryStore) {
	store := &memoryStore{data: map[string][]byte{}}
	svc := cacheapp.NewSemanticCacheService(fakeEmbedder{}, store, time.Minute, 0.9)
	return newCacheTestEngineExt(enabled, calls, svc, store), store
}

func doChatRequest(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBufferString(body))
	engine.ServeHTTP(w, req)
	return w
}

const maskedBody = `{"model":"gpt-4","messages":[{"role":"user","content":"[MASK_ab.0] how much?"}]}`

// @sk-test semantic-cache-masked#T2.4: First request misses, response is stored (AC-002)
func TestSemanticCacheMissStores(t *testing.T) {
	engine, store := newCacheTestEngine(true, new(int))
	resp := doChatRequest(engine, maskedBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if len(store.keys()) != 1 {
		t.Fatalf("expected 1 stored entry, got %d", len(store.keys()))
	}
}

// @sk-test semantic-cache-masked#T2.4: Second equivalent request hits without calling provider (AC-001)
func TestSemanticCacheHitSkipsProvider(t *testing.T) {
	calls := 0
	engine, store := newCacheTestEngine(true, &calls)
	first := doChatRequest(engine, maskedBody)
	if len(store.keys()) != 1 {
		t.Fatalf("expected 1 stored entry after miss, got %d", len(store.keys()))
	}
	if calls != 1 {
		t.Fatalf("expected provider called once on miss, got %d", calls)
	}

	second := doChatRequest(engine, maskedBody)
	if second.Code != http.StatusOK {
		t.Fatalf("expected 200 on hit, got %d", second.Code)
	}
	if calls != 1 {
		t.Fatalf("provider called %d times, expected 1 (second request must hit cache)", calls)
	}
	if !bytes.Equal(second.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("hit body differs from stored masked body:\n got  %s\n want %s", second.Body.Bytes(), first.Body.Bytes())
	}
}

// @sk-test semantic-cache-masked#T2.4: Disabled cache always calls provider (AC-007)
func TestSemanticCacheDisabledPassesThrough(t *testing.T) {
	engine, store := newCacheTestEngine(false, new(int))
	first := doChatRequest(engine, maskedBody)
	second := doChatRequest(engine, maskedBody)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("expected 200s, got %d %d", first.Code, second.Code)
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatalf("disabled cache must pass through identical provider responses")
	}
	if len(store.keys()) != 0 {
		t.Fatalf("disabled cache must not write entries, got %d keys", len(store.keys()))
	}
}

// @sk-test semantic-cache-masked#T2.4: Streaming request bypasses cache (RQ-010 variant A)
func TestSemanticCacheSkipsStream(t *testing.T) {
	calls := 0
	engine, store := newCacheTestEngine(true, &calls)
	body := `{"model":"gpt-4","stream":true,"messages":[{"role":"user","content":"[MASK_ab.0] hi"}]}`
	first := doChatRequest(engine, body)
	second := doChatRequest(engine, body)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("expected 200s, got %d %d", first.Code, second.Code)
	}
	if calls != 2 {
		t.Fatalf("streaming must bypass cache, provider called %d times", calls)
	}
	if len(store.keys()) != 0 {
		t.Fatalf("streaming must not write cache, got %d keys", len(store.keys()))
	}
}

// @sk-test semantic-cache-masked#T2.4: Cached value is valid JSON (AC-001)
func TestSemanticCacheHitIsValidJSON(t *testing.T) {
	engine, _ := newCacheTestEngine(true, new(int))
	_ = doChatRequest(engine, maskedBody)
	hit := doChatRequest(engine, maskedBody)
	var parsed map[string]any
	if err := json.Unmarshal(hit.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("hit body is not valid JSON: %v", err)
	}
}

// --- Phase 3: fallback embedder, budget guard, disable ---

type errorEmbedder struct{}

func (errorEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, errors.New("embedding endpoint unavailable")
}

// @sk-test semantic-cache-masked#T3.1: Unavailable external falls back to offline; client 2xx (AC-006)
func TestSemanticCacheFallbackEmbedderDegrades(t *testing.T) {
	metrics.CacheErrorsTotal.WithLabelValues("alpha").Add(0)
	before := testutil.ToFloat64(metrics.CacheErrorsTotal.WithLabelValues("alpha"))

	calls := 0
	store := &memoryStore{data: map[string][]byte{}}
	fallback := cacheapp.NewFallbackEmbedder(errorEmbedder{}, fakeEmbedder{})
	svc := cacheapp.NewSemanticCacheService(fallback, store, time.Minute, 0.9)
	engine := newCacheTestEngineExt(true, &calls, svc, store)

	resp := doChatRequest(engine, maskedBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 even with unavailable embedder, got %d", resp.Code)
	}
	if calls != 1 {
		t.Fatalf("expected provider called on miss, got %d", calls)
	}
	after := testutil.ToFloat64(metrics.CacheErrorsTotal.WithLabelValues("alpha"))
	if after <= before {
		t.Fatalf("expected cache error metric to increase on fallback (before=%v after=%v)", before, after)
	}

	hit := doChatRequest(engine, maskedBody)
	if hit.Code != http.StatusOK {
		t.Fatalf("expected 200 on hit, got %d", hit.Code)
	}
	if calls != 1 {
		t.Fatalf("expected second request to hit cache, provider called %d times", calls)
	}
}

// @sk-test semantic-cache-masked#T3.2: Guard blocks writes near hard limit but serves hits (AC-008)
func TestSemanticCacheBudgetGuardBlocksWrites(t *testing.T) {
	calls := 0

	// Phase 1: no guard, seed the cache for text A.
	store := &memoryStore{data: map[string][]byte{}}
	seedSvc := cacheapp.NewSemanticCacheService(fakeEmbedder{}, store, time.Minute, 0.9)
	seedEngine := newCacheTestEngineExt(true, &calls, seedSvc, store)
	first := doChatRequest(seedEngine, maskedBody)
	if first.Code != http.StatusOK || calls != 1 {
		t.Fatalf("seed miss failed: code=%d calls=%d", first.Code, calls)
	}
	if len(store.keys()) != 1 {
		t.Fatalf("expected 1 seeded key, got %d", len(store.keys()))
	}

	// Phase 2: guard active, tenant near hard limit (spent=96 >= 95 with guard 5%).
	b, _ := budget.NewBudget("b1", "alpha", budget.ScopeTenant, budget.PeriodMonthly)
	hard := 100.0
	b.HardLimit = &hard
	counter := &fakeBudgetCounter{values: map[string]float64{b.CounterKey(testNow()): 96}}
	guard := cacheapp.NewBudgetWriteGuard(&fakeBudgetRepo{budgets: []*budget.Budget{b}}, counter, 5)
	guardedSvc := cacheapp.NewSemanticCacheService(fakeEmbedder{}, store, time.Minute, 0.9).WithGuard(guard)
	guardedEngine := newCacheTestEngineExt(true, &calls, guardedSvc, store)

	// Hit for seeded text A still works under guard.
	hit := doChatRequest(guardedEngine, maskedBody)
	if hit.Code != http.StatusOK {
		t.Fatalf("expected 200 on hit under guard, got %d", hit.Code)
	}
	if calls != 1 {
		t.Fatalf("expected hit to skip provider under guard, provider called %d times", calls)
	}

	// New text B is a miss: provider called, but write must be blocked.
	beforeKeys := len(store.keys())
	bodyB := `{"model":"gpt-4","messages":[{"role":"user","content":"[MASK_ab.1] different question"}]}`
	miss := doChatRequest(guardedEngine, bodyB)
	if miss.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", miss.Code)
	}
	if calls != 2 {
		t.Fatalf("expected provider called on guarded miss, got %d", calls)
	}
	if len(store.keys()) != beforeKeys {
		t.Fatalf("guard must block new writes, keys before=%d after=%d", beforeKeys, len(store.keys()))
	}
	if testutil.ToFloat64(metrics.CacheWriteBlockedTotal.WithLabelValues("alpha")) == 0 {
		t.Fatal("expected cache write blocked metric to be set")
	}
}
