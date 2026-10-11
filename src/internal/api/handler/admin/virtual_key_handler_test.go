package admin

import (
	"bytes"
	"context"
	"encoding/base64"
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
	infraCrypto "github.com/bzdvdn/maskchain/src/internal/infra/crypto"
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

// @sk-task ui-production-readiness#T7.1: DB-level pagination/search (AC-008)
func (f *fakeVirtualKeyRepo) ListPaged(_ context.Context, limit, offset int, search string) ([]*virtualkey.VirtualKey, int, error) {
	filtered := make([]*virtualkey.VirtualKey, 0, len(f.keys))
	for _, k := range f.keys {
		if search == "" || containsFold(k.ID, search) || containsFold(k.TenantID, search) || containsFold(k.Label, search) {
			filtered = append(filtered, k)
		}
	}
	total := len(filtered)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
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

func (f *fakeVirtualKeyRepo) Purge(_ context.Context, id string) error {
	for i, k := range f.keys {
		if k.ID == id {
			f.keys = append(f.keys[:i], f.keys[i+1:]...)
			return nil
		}
	}
	return virtualkey.ErrNotFound
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

type recordAudit struct{ events []*AuditEvent }

func (r *recordAudit) Write(_ context.Context, e *AuditEvent) error {
	r.events = append(r.events, e)
	return nil
}

func testEncryptor(t *testing.T) *infraCrypto.Encryptor {
	t.Helper()
	enc, err := infraCrypto.New(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatalf("build encryptor: %v", err)
	}
	return enc
}

// @sk-test key-secret-reveal#T4.1: create + reveal returns the same secret and audits (AC-001, AC-006)
func TestVirtualKeyHandlerRevealStoredSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{}
	audit := &recordAudit{}
	h := NewVirtualKeyHandlerWithEncryptor(repo, audit, testEncryptor(t))
	router := gin.New()
	router.POST("/api/v1/keys", h.Create)
	router.GET("/api/v1/keys/:id/secret", h.Reveal)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewBufferString(`{"tenant_id":"acme","label":"prod"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Key == "" {
		t.Fatal("expected a plaintext secret at creation")
	}

	stored, err := repo.GetById(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if stored.SecretEncrypted == "" || stored.SecretEncrypted == created.Key {
		t.Fatalf("expected stored secret to be ciphertext, got %q", stored.SecretEncrypted)
	}

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/keys/"+created.ID+"/secret", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 reveal, got %d: %s", w2.Code, w2.Body.String())
	}
	var revealed struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &revealed); err != nil {
		t.Fatalf("decode reveal: %v", err)
	}
	if revealed.Key != created.Key {
		t.Errorf("expected revealed secret %q, got %q", created.Key, revealed.Key)
	}

	var audited bool
	for _, e := range audit.events {
		if e.Action == "reveal_key_secret" && e.Target == created.ID {
			audited = true
		}
	}
	if !audited {
		t.Error("expected a reveal_key_secret audit entry")
	}
}

// @sk-test key-secret-reveal#T4.1: rotate refreshes the revealable secret (AC-003)
func TestVirtualKeyHandlerRevealAfterRotate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{}
	h := NewVirtualKeyHandlerWithEncryptor(repo, nil, testEncryptor(t))
	router := gin.New()
	router.POST("/api/v1/keys", h.Create)
	router.POST("/api/v1/keys/:id/rotate", h.Rotate)
	router.GET("/api/v1/keys/:id/secret", h.Reveal)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewBufferString(`{"tenant_id":"acme"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/v1/keys/"+created.ID+"/rotate", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200 rotate, got %d: %s", rw.Code, rw.Body.String())
	}
	var rotated struct {
		Key string `json:"key"`
	}
	_ = json.Unmarshal(rw.Body.Bytes(), &rotated)
	if rotated.Key == "" || rotated.Key == created.Key {
		t.Fatalf("expected a new secret after rotate, got %q", rotated.Key)
	}

	vw := httptest.NewRecorder()
	router.ServeHTTP(vw, httptest.NewRequest(http.MethodGet, "/api/v1/keys/"+created.ID+"/secret", nil))
	var revealed struct {
		Key string `json:"key"`
	}
	_ = json.Unmarshal(vw.Body.Bytes(), &revealed)
	if revealed.Key != rotated.Key {
		t.Errorf("expected reveal to return the rotated secret %q, got %q", rotated.Key, revealed.Key)
	}
}

// @sk-test key-secret-reveal#T4.1: legacy/unavailable secret returns 409 SECRET_UNAVAILABLE (AC-004)
func TestVirtualKeyHandlerRevealUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{adminFakeKey("k1", "hash-1", "acme")}}
	h := NewVirtualKeyHandlerWithEncryptor(repo, nil, testEncryptor(t))
	router := gin.New()
	router.GET("/api/v1/keys/:id/secret", h.Reveal)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/keys/k1/secret", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error.Code != "SECRET_UNAVAILABLE" {
		t.Errorf("expected SECRET_UNAVAILABLE, got %q", errResp.Error.Code)
	}
}

// @sk-test key-secret-reveal#T4.1: nil encryptor stores nothing and degrades to 409 (AC-005)
func TestVirtualKeyHandlerRevealNilEncryptor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{}
	h := NewVirtualKeyHandler(repo, nil)
	router := gin.New()
	router.POST("/api/v1/keys", h.Create)
	router.GET("/api/v1/keys/:id/secret", h.Reveal)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewBufferString(`{"tenant_id":"acme"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 create without encryptor, got %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	stored, _ := repo.GetById(context.Background(), created.ID)
	if stored.SecretEncrypted != "" {
		t.Errorf("expected no stored secret without an encryptor, got %q", stored.SecretEncrypted)
	}

	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/v1/keys/"+created.ID+"/secret", nil))
	if rw.Code != http.StatusConflict {
		t.Fatalf("expected 409 without encryptor, got %d: %s", rw.Code, rw.Body.String())
	}
}

// @sk-test key-permanent-delete#T3.1: purge removes the key, invalidates the cache, audits (AC-001, AC-003, AC-004)
func TestVirtualKeyHandlerPurge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{keys: []*virtualkey.VirtualKey{adminFakeKey("k1", "hash-1", "acme")}}
	audit := &recordAudit{}
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := middleware.NewVirtualKeyCache(repo, logger)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("warmup: %v", err)
	}

	h := NewVirtualKeyHandler(repo, audit, cache)
	router := gin.New()
	router.DELETE("/api/v1/keys/:id/purge", h.Purge)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/keys/k1/purge", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := repo.GetById(context.Background(), "k1"); err == nil {
		t.Error("expected the key to be removed from the repository")
	}
	if _, ok := cache.Get("hash-1"); ok {
		t.Error("expected the purged key to be invalidated in the cache")
	}

	var audited bool
	for _, e := range audit.events {
		if e.Action == "purge_key" && e.Target == "k1" {
			audited = true
		}
	}
	if !audited {
		t.Error("expected a purge_key audit entry")
	}
}

// @sk-test key-permanent-delete#T3.1: purge of an unknown id returns 404 (AC-002)
func TestVirtualKeyHandlerPurgeNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeVirtualKeyRepo{}
	h := NewVirtualKeyHandler(repo, nil)
	router := gin.New()
	router.DELETE("/api/v1/keys/:id/purge", h.Purge)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/keys/missing/purge", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
