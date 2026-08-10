package conversation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
)

type mockConversationStore struct {
	mu    sync.Mutex
	items map[string]conversation.ConversationLog
	err   error
}

func newMockConversationStore() *mockConversationStore {
	return &mockConversationStore{items: make(map[string]conversation.ConversationLog)}
}

func (m *mockConversationStore) SaveBatch(_ context.Context, logs []conversation.ConversationLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range logs {
		m.items[l.ID] = l
	}
	return nil
}

func (m *mockConversationStore) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (m *mockConversationStore) List(_ context.Context, filter conversation.ConversationFilter) (conversation.ConversationPage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return conversation.ConversationPage{}, m.err
	}
	var all []conversation.ConversationLog
	for _, l := range m.items {
		if filter.TenantID != "" && l.TenantID != filter.TenantID {
			continue
		}
		all = append(all, l)
	}
	// metadata only: strip content
	for i := range all {
		all[i].Request = nil
		all[i].Response = nil
		all[i].Masking = nil
	}
	return conversation.ConversationPage{Items: all, Total: len(all)}, nil
}

func (m *mockConversationStore) Get(_ context.Context, id string) (*conversation.ConversationLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	l, ok := m.items[id]
	if !ok {
		return nil, conversation.ErrNotFound
	}
	copy := l
	return &copy, nil
}

func testEncryptor(t *testing.T) *crypto.Encryptor {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	enc, err := crypto.New(key)
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return enc
}

func newTestConversationHandler(t *testing.T, store conversation.ConversationStore) *ConversationHandler {
	t.Helper()
	return NewConversationHandler(store, testEncryptor(t))
}

func setupConversationTestEngine(t *testing.T, h *ConversationHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	group := engine.Group("/api/v1/conversations")
	group.GET("", h.HandleList)
	group.GET("/:id", h.HandleGet)
	return engine
}

// @sk-test conversation-logging#T3.1: TestConversationHandlerListMetadataOnly (AC-006)
func TestConversationHandlerListMetadataOnly(t *testing.T) {
	store := newMockConversationStore()
	now := time.Now().UTC()
	for i, id := range []string{"id-1", "id-2"} {
		l := conversation.ConversationLog{
			ID:         id,
			TenantID:   "tenant-a",
			Model:      "gpt-4o",
			Status:     conversation.StatusOK,
			Masked:     i == 0,
			Streamed:   i == 1,
			Request:    []byte("req-cipher"),
			RequestLen: 10,
			MaskID:     "MASK_" + id,
			CreatedAt:  now.Add(-time.Duration(i) * time.Hour),
		}
		if err := store.SaveBatch(context.Background(), []conversation.ConversationLog{l}); err != nil {
			t.Fatalf("seed store: %v", err)
		}
	}

	engine := setupConversationTestEngine(t, newTestConversationHandler(t, store))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations?tenant_id=tenant-a&page=1&per_page=10", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data struct {
			Items []dtoConversationListItem `json:"items"`
		} `json:"data"`
		Pagination struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Data.Items))
	}
	if resp.Pagination.Total != 2 {
		t.Errorf("pagination total = %d, want 2", resp.Pagination.Total)
	}
	for _, item := range resp.Data.Items {
		if item.Request != "" || item.Response != "" || item.Masking != "" {
			t.Errorf("list must not contain payload content: %+v", item)
		}
	}
	if !resp.Data.Items[0].Masked || !resp.Data.Items[1].Streamed {
		t.Errorf("metadata fields not mapped: %+v", resp.Data.Items)
	}
	if resp.Data.Items[0].MaskID != "MASK_id-1" {
		t.Errorf("MaskID = %q, want MASK_id-1", resp.Data.Items[0].MaskID)
	}
}

type dtoConversationListItem struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Model    string `json:"model"`
	Status   string `json:"status"`
	Masked   bool   `json:"masked"`
	Streamed bool   `json:"streamed"`
	MaskID   string `json:"mask_id,omitempty"`
	Request  string `json:"request,omitempty"`
	Response string `json:"response,omitempty"`
	Masking  string `json:"masking,omitempty"`
}

// @sk-test conversation-logging#T3.1: TestConversationHandlerEnvelopeNotDoubleWrapped (AC-005, AC-006)
func TestConversationHandlerEnvelopeNotDoubleWrapped(t *testing.T) {
	store := newMockConversationStore()
	now := time.Now().UTC()
	enc := testEncryptor(t)
	reqCt, err := enc.Encrypt([]byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatalf("encrypt req: %v", err)
	}
	l := conversation.ConversationLog{
		ID:         "id-1",
		TenantID:   "tenant-a",
		Model:      "gpt-4o",
		Status:     conversation.StatusOK,
		Masked:     true,
		Request:    reqCt,
		RequestLen: len(reqCt),
		MaskID:     "MASK_id-1",
		CreatedAt:  now,
	}
	if err := store.SaveBatch(context.Background(), []conversation.ConversationLog{l}); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := store.SaveBatch(context.Background(), []conversation.ConversationLog{l}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.ResponseEnvelope())
	engine.Use(middleware.ErrorHandler())
	group := engine.Group("/api/v1/conversations")
	h := newTestConversationHandler(t, store)
	group.GET("", h.HandleList)
	group.GET("/:id", h.HandleGet)

	t.Run("list is single-enveloped", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil)
		engine.ServeHTTP(w, req)

		var resp struct {
			Data struct {
				Items []dtoConversationListItem `json:"items"`
			} `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(resp.Data.Items) != 1 {
			t.Errorf("expected 1 item at data.items, got %d (double envelope?)", len(resp.Data.Items))
		}
		if resp.Pagination.Total != 1 {
			t.Errorf("pagination total = %d, want 1", resp.Pagination.Total)
		}
	})

	t.Run("detail is single-enveloped", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/id-1", nil)
		engine.ServeHTTP(w, req)

		var resp struct {
			Data struct {
				ID     string `json:"id"`
				MaskID string `json:"mask_id,omitempty"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Data.ID != "id-1" {
			t.Errorf("expected id at data.id, got %q (double envelope?)", resp.Data.ID)
		}
		if resp.Data.MaskID != "MASK_id-1" {
			t.Errorf("MaskID = %q, want MASK_id-1", resp.Data.MaskID)
		}
	})
}

// @sk-test conversation-logging#T3.1: TestConversationHandlerDetailBase64RoundTrip (AC-005)
func TestConversationHandlerDetailBase64RoundTrip(t *testing.T) {
	enc := testEncryptor(t)
	store := newMockConversationStore()

	reqPlain := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`)
	respPlain := []byte(`data: {"content":"hi"}`)
	maskingPlain := []byte(`[{"placeholder":"[MASK_A.0]","original":"secret@example.com"}]`)

	req, err := enc.Encrypt(reqPlain)
	if err != nil {
		t.Fatalf("encrypt req: %v", err)
	}
	resp, err := enc.Encrypt(respPlain)
	if err != nil {
		t.Fatalf("encrypt resp: %v", err)
	}
	masking, err := enc.Encrypt(maskingPlain)
	if err != nil {
		t.Fatalf("encrypt masking: %v", err)
	}

	l := conversation.ConversationLog{
		ID:        "id-1",
		TenantID:  "tenant-a",
		Model:     "gpt-4o",
		Status:    conversation.StatusOK,
		Masked:    true,
		Streamed:  true,
		MaskID:    "MASK_DETAIL_A",
		Request:   req,
		Response:  resp,
		Masking:   masking,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.SaveBatch(context.Background(), []conversation.ConversationLog{l}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	handler := NewConversationHandler(store, enc)
	engine := setupConversationTestEngine(t, handler)
	w := httptest.NewRecorder()
	reqHTTP := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/id-1", nil)
	engine.ServeHTTP(w, reqHTTP)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var respEnv struct {
		Data struct {
			ID       string `json:"id"`
			Streamed bool   `json:"streamed"`
			Masked   bool   `json:"masked"`
			MaskID   string `json:"mask_id,omitempty"`
			Payload  struct {
				Request  string `json:"request"`
				Response string `json:"response"`
				Masking  string `json:"masking"`
			} `json:"payload"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &respEnv); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if respEnv.Data.ID != "id-1" {
		t.Errorf("id = %q", respEnv.Data.ID)
	}
	if !respEnv.Data.Streamed || !respEnv.Data.Masked {
		t.Errorf("flags not surfaced: %+v", respEnv.Data)
	}
	if respEnv.Data.MaskID != "MASK_DETAIL_A" {
		t.Errorf("MaskID = %q, want MASK_DETAIL_A", respEnv.Data.MaskID)
	}

	gotReq, err := base64.StdEncoding.DecodeString(respEnv.Data.Payload.Request)
	if err != nil {
		t.Fatalf("decode request b64: %v", err)
	}
	if string(gotReq) != string(reqPlain) {
		t.Errorf("request = %q, want %q", gotReq, reqPlain)
	}
	gotResp, err := base64.StdEncoding.DecodeString(respEnv.Data.Payload.Response)
	if err != nil {
		t.Fatalf("decode response b64: %v", err)
	}
	if string(gotResp) != string(respPlain) {
		t.Errorf("response = %q, want %q", gotResp, respPlain)
	}
	gotMasking, err := base64.StdEncoding.DecodeString(respEnv.Data.Payload.Masking)
	if err != nil {
		t.Fatalf("decode masking b64: %v", err)
	}
	if string(gotMasking) != string(maskingPlain) {
		t.Errorf("masking = %q, want %q", gotMasking, maskingPlain)
	}
}

// @sk-test conversation-logging#T3.1: TestConversationHandlerDetailNotFound (AC-005)
func TestConversationHandlerDetailNotFound(t *testing.T) {
	store := newMockConversationStore()
	engine := setupConversationTestEngine(t, newTestConversationHandler(t, store))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/missing", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// @sk-test conversation-logging#T3.1: TestConversationHandlerDetailDecryptError (AC-005)
func TestConversationHandlerDetailDecryptError(t *testing.T) {
	store := newMockConversationStore()
	l := conversation.ConversationLog{
		ID:        "id-1",
		TenantID:  "tenant-a",
		Model:     "gpt-4o",
		Status:    conversation.StatusOK,
		Request:   []byte("not-ciphertext"),
		CreatedAt: time.Now().UTC(),
	}
	if err := store.SaveBatch(context.Background(), []conversation.ConversationLog{l}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	engine := setupConversationTestEngine(t, newTestConversationHandler(t, store))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/id-1", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
