package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// fakeVirtualKeyRepo is the admin-package in-memory virtual key repository used
// by handler tests (mirrors middleware.fakeVirtualKeyRepo).
type fakeVirtualKeyRepo struct {
	keys []*virtualkey.VirtualKey
}

func (f *fakeVirtualKeyRepo) FindByKeyHash(_ context.Context, hash string) (*virtualkey.VirtualKey, error) {
	for _, k := range f.keys {
		if k.KeyHash == hash && k.Enabled {
			return k, nil
		}
	}
	return nil, virtualkey.ErrNotFound
}

func (f *fakeVirtualKeyRepo) GetById(_ context.Context, id string) (*virtualkey.VirtualKey, error) {
	for _, k := range f.keys {
		if k.ID == id {
			return k, nil
		}
	}
	return nil, virtualkey.ErrNotFound
}

func (f *fakeVirtualKeyRepo) ListByTenant(_ context.Context, tenantID string) ([]*virtualkey.VirtualKey, error) {
	var out []*virtualkey.VirtualKey
	for _, k := range f.keys {
		if k.TenantID == tenantID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeVirtualKeyRepo) List(_ context.Context) ([]*virtualkey.VirtualKey, error) {
	return f.keys, nil
}

func (f *fakeVirtualKeyRepo) Create(_ context.Context, k *virtualkey.VirtualKey) error {
	f.keys = append(f.keys, k)
	return nil
}

func (f *fakeVirtualKeyRepo) Update(_ context.Context, k *virtualkey.VirtualKey) error {
	for i, item := range f.keys {
		if item.ID == k.ID {
			f.keys[i] = k
			return nil
		}
	}
	return virtualkey.ErrNotFound
}

func (f *fakeVirtualKeyRepo) Delete(_ context.Context, id string) error {
	for i, k := range f.keys {
		if k.ID == id {
			f.keys[i].Enabled = false
			return nil
		}
	}
	return virtualkey.ErrNotFound
}

func (f *fakeVirtualKeyRepo) BackfillFromLegacy(_ context.Context, _ map[string][]string) (int, error) {
	return 0, nil
}

func adminFakeKey(id, hash, tenantID string) *virtualkey.VirtualKey {
	now := time.Now().UTC()
	return &virtualkey.VirtualKey{
		ID:        id,
		TenantID:  tenantID,
		KeyHash:   hash,
		Label:     "legacy",
		Metadata:  map[string]string{},
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyHandlerCreateInvalidatesCache (AC-005, AC-007)
func TestVirtualKeyHandlerCreateInvalidatesCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{adminFakeKey("old-id", "old-hash", "acme")}}
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := middleware.NewVirtualKeyCache(repo, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("warmup: %v", err)
	}
	if cache.Len() != 1 {
		t.Fatalf("expected 1 warm key, got %d", cache.Len())
	}

	h := NewVirtualKeyHandler(repo, nil, cache)
	router := gin.New()
	router.POST("/api/v1/keys", h.Create)

	body := `{"tenant_id":"acme","label":"created-key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if cache.Len() != 1 {
		t.Errorf("expected create to invalidate the new key (cache stays at old count), got %d", cache.Len())
	}
	if _, ok := cache.Get("old-hash"); !ok {
		t.Error("expected pre-existing key to remain cached after unrelated create")
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyHandlerDeleteInvalidatesCache (AC-005, AC-007)
func TestVirtualKeyHandlerDeleteInvalidatesCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{adminFakeKey("k1", "hash-1", "acme")}}
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := middleware.NewVirtualKeyCache(repo, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("warmup: %v", err)
	}
	if cache.Len() != 1 {
		t.Fatalf("expected 1 warm key, got %d", cache.Len())
	}

	h := NewVirtualKeyHandler(repo, nil, cache)
	router := gin.New()
	router.DELETE("/api/v1/keys/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/keys/k1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := cache.Get("hash-1"); ok {
		t.Error("expected revoked key to be removed from cache")
	}
	if cache.Len() != 0 {
		t.Errorf("expected empty cache after delete, got %d", cache.Len())
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyHandlerUpdateInvalidatesCache (AC-005, AC-007)
func TestVirtualKeyHandlerUpdateInvalidatesCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{adminFakeKey("k1", "hash-1", "acme")}}
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := middleware.NewVirtualKeyCache(repo, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("warmup: %v", err)
	}

	h := NewVirtualKeyHandler(repo, nil, cache)
	router := gin.New()
	router.PUT("/api/v1/keys/:id", h.Update)

	body := `{"enabled":false,"label":"disabled"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/keys/k1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := cache.Get("hash-1"); ok {
		t.Error("expected updated key to be invalidated in cache")
	}
	if cache.Len() != 0 {
		t.Errorf("expected empty cache after update, got %d", cache.Len())
	}
}

// @sk-test api-self-service: TestVirtualKeyHandlerRotate (AC-001)
func TestVirtualKeyHandlerRotate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := adminFakeKey("k1", "hash-old", "acme")
	old.Label = "prod-app"
	old.AllowedModels = []string{"gpt-4o-mini"}
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{old}}
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := middleware.NewVirtualKeyCache(repo, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("warmup: %v", err)
	}

	h := NewVirtualKeyHandler(repo, nil, cache)
	router := gin.New()
	router.POST("/api/v1/keys/:id/rotate", h.Rotate)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/k1/rotate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID            string   `json:"id"`
		Label         string   `json:"label"`
		AllowedModels []string `json:"allowed_models"`
		Key           string   `json:"key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Key == "" {
		t.Fatal("expected a new plaintext key in the response")
	}
	if !strings.HasPrefix(resp.Key, virtualkey.KeyPrefix) {
		t.Errorf("expected key prefix %q, got %q", virtualkey.KeyPrefix, resp.Key)
	}
	if resp.ID != "k1" || resp.Label != "prod-app" {
		t.Errorf("expected id/label preserved, got %q/%q", resp.ID, resp.Label)
	}
	if len(resp.AllowedModels) != 1 || resp.AllowedModels[0] != "gpt-4o-mini" {
		t.Errorf("expected scopes preserved, got %v", resp.AllowedModels)
	}

	stored, err := repo.GetById(context.Background(), "k1")
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if stored.KeyHash == "hash-old" {
		t.Error("expected stored hash to change after rotation")
	}
	if stored.KeyHash != virtualkey.KeyHash(resp.Key) {
		t.Error("expected stored hash to match the returned secret")
	}
	if _, ok := cache.Get("hash-old"); ok {
		t.Error("expected the old hash to be invalidated in cache")
	}
}

// @sk-test api-self-service: TestVirtualKeyHandlerRotateNotFound (AC-001)
func TestVirtualKeyHandlerRotateNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{}
	h := NewVirtualKeyHandler(repo, nil)
	router := gin.New()
	router.POST("/api/v1/keys/:id/rotate", h.Rotate)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/missing/rotate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestVirtualKeyHandlerWithoutCache (AC-005, AC-006)
func TestVirtualKeyHandlerWithoutCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{}
	h := NewVirtualKeyHandler(repo, nil)
	router := gin.New()
	router.POST("/api/v1/keys", h.Create)

	body := `{"tenant_id":"acme","label":"no-cache"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 without cache, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.keys) != 1 {
		t.Errorf("expected 1 key created, got %d", len(repo.keys))
	}
}
