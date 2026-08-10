package conversation

import (
	"context"
	"sync"
	"testing"
	"time"
)

// @sk-test conversation-logging#T1.3: TestConversationLog (AC-001, AC-002)
func TestConversationLog(t *testing.T) {
	now := time.Now().UTC()

	t.Run("creates ok record", func(t *testing.T) {
		log, err := NewConversationLog("id-1", "tenant-1", "gpt-4o", StatusOK, []byte("req"), now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if log.ID != "id-1" || log.TenantID != "tenant-1" || log.Model != "gpt-4o" {
			t.Error("fields not set")
		}
		if log.RequestLen != 3 {
			t.Errorf("RequestLen = %d, want 3", log.RequestLen)
		}
		if log.Masked {
			t.Error("expected Masked=false by default")
		}
	})

	t.Run("rejects empty id", func(t *testing.T) {
		_, err := NewConversationLog("", "t", "m", StatusOK, []byte("r"), now)
		if err == nil {
			t.Fatal("expected error for empty id")
		}
	})

	t.Run("rejects empty request", func(t *testing.T) {
		_, err := NewConversationLog("id", "t", "m", StatusOK, nil, now)
		if err == nil {
			t.Fatal("expected error for empty request")
		}
	})

	t.Run("rejects invalid status", func(t *testing.T) {
		_, err := NewConversationLog("id", "t", "m", "wat", []byte("r"), now)
		if err == nil {
			t.Fatal("expected error for invalid status")
		}
	})

	t.Run("with response and masking sets lengths and flag", func(t *testing.T) {
		log, err := NewConversationLog("id", "tenant-1", "gpt-4o", StatusOK, []byte("request body"), now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		log.WithResponse([]byte("response body"))
		if log.ResponseLen != 13 {
			t.Errorf("ResponseLen = %d, want 13", log.ResponseLen)
		}

		entries := []MaskingEntry{{Placeholder: "[MASK_1]", Original: "secret@example.com"}}
		encryptedMasking := []byte("ciphertext-masking")
		log.WithMasking(encryptedMasking)
		if !log.Masked {
			t.Error("expected Masked=true after WithMasking")
		}
		if string(log.Masking) != "ciphertext-masking" {
			t.Errorf("Masking = %q, want %q", log.Masking, "ciphertext-masking")
		}

		encoded, err := MaskingEntriesJSON(entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(encoded) != `[{"placeholder":"[MASK_1]","original":"secret@example.com"}]` {
			t.Errorf("unexpected masking JSON: %s", encoded)
		}

		parsed, err := ParseMaskingEntriesJSON(encoded)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(parsed) != 1 || parsed[0].Placeholder != "[MASK_1]" {
			t.Errorf("unexpected parsed entries: %+v", parsed)
		}
	})

	t.Run("empty masking yields nil and Masked=false", func(t *testing.T) {
		log, _ := NewConversationLog("id", "t", "m", StatusOK, []byte("r"), now)
		log.WithMasking(nil)
		if log.Masked {
			t.Error("expected Masked=false for empty masking")
		}
		encoded, err := MaskingEntriesJSON(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if encoded != nil {
			t.Errorf("expected nil JSON for empty masking, got %q", encoded)
		}
	})
}

// @sk-test conversation-logging#T1.3: TestConversationStoreInterface (AC-001, AC-003)
func TestConversationStoreInterface(t *testing.T) {
	var _ ConversationStore = (*memoryConversationStore)(nil)
}

// @sk-test conversation-logging#T1.3: TestMemoryConversationStore (AC-001, AC-005, AC-006, AC-007)
func TestMemoryConversationStore(t *testing.T) {
	store := newMemoryConversationStore()
	ctx := context.Background()
	now := time.Now().UTC()

	old := mustLog(t, "old-1", "tenant-1", "gpt-4o", StatusOK, now.Add(-48*time.Hour))
	recent := mustLog(t, "new-1", "tenant-1", "gpt-4o", StatusOK, now.Add(-time.Hour))
	otherTenant := mustLog(t, "other-1", "tenant-2", "claude", StatusOK, now.Add(-time.Minute))

	if err := store.SaveBatch(ctx, []ConversationLog{old, recent, otherTenant}); err != nil {
		t.Fatalf("save error: %v", err)
	}

	t.Run("save is idempotent", func(t *testing.T) {
		if err := store.SaveBatch(ctx, []ConversationLog{recent}); err != nil {
			t.Fatalf("re-save error: %v", err)
		}
	})

	t.Run("list is metadata only", func(t *testing.T) {
		page, err := store.List(ctx, ConversationFilter{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("list error: %v", err)
		}
		if page.Total != 3 {
			t.Errorf("Total = %d, want 3", page.Total)
		}
		if len(page.Items) != 3 {
			t.Errorf("items = %d, want 3", len(page.Items))
		}
	})

	t.Run("list filters by tenant", func(t *testing.T) {
		page, err := store.List(ctx, ConversationFilter{TenantID: "tenant-1", Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("list error: %v", err)
		}
		if page.Total != 2 {
			t.Errorf("Total = %d, want 2", page.Total)
		}
	})

	t.Run("list paginates", func(t *testing.T) {
		page, err := store.List(ctx, ConversationFilter{Page: 1, PerPage: 2})
		if err != nil {
			t.Fatalf("list error: %v", err)
		}
		if len(page.Items) != 2 || page.Total != 3 {
			t.Errorf("page len=%d total=%d, want 2/3", len(page.Items), page.Total)
		}
	})

	t.Run("get returns content", func(t *testing.T) {
		got, err := store.Get(ctx, "new-1")
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		if string(got.Request) != "request body" {
			t.Errorf("request = %q, want %q", got.Request, "request body")
		}
	})

	t.Run("get missing returns error", func(t *testing.T) {
		if _, err := store.Get(ctx, "missing"); err == nil {
			t.Fatal("expected error for missing record")
		}
	})

	t.Run("delete older than TTL keeps recent", func(t *testing.T) {
		deleted, err := store.DeleteOlderThan(ctx, now.Add(-24*time.Hour))
		if err != nil {
			t.Fatalf("delete error: %v", err)
		}
		if deleted != 1 {
			t.Errorf("deleted = %d, want 1", deleted)
		}
		page, err := store.List(ctx, ConversationFilter{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("list error: %v", err)
		}
		if page.Total != 2 {
			t.Errorf("Total after cleanup = %d, want 2", page.Total)
		}
	})
}

// @sk-test conversation-logging#T2.4: TestMemoryConversationStoreStreamedRoundTrip (RQ-009, DEC-007, AC-003)
func TestMemoryConversationStoreStreamedRoundTrip(t *testing.T) {
	store := newMemoryConversationStore()
	ctx := context.Background()

	streamed := mustLog(t, "stream-1", "tenant-1", "gpt-4o", StatusOK, time.Now().UTC())
	streamed.Streamed = true
	plain := mustLog(t, "plain-1", "tenant-1", "gpt-4o", StatusOK, time.Now().UTC())

	if err := store.SaveBatch(ctx, []ConversationLog{streamed, plain}); err != nil {
		t.Fatalf("save error: %v", err)
	}

	t.Run("get returns streamed flag", func(t *testing.T) {
		got, err := store.Get(ctx, "stream-1")
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		if !got.Streamed {
			t.Error("expected Streamed=true for streamed record")
		}
		gotPlain, err := store.Get(ctx, "plain-1")
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		if gotPlain.Streamed {
			t.Error("expected Streamed=false for plain record")
		}
	})

	t.Run("list surfaces streamed flag", func(t *testing.T) {
		page, err := store.List(ctx, ConversationFilter{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("list error: %v", err)
		}
		var streamedCount int
		for _, item := range page.Items {
			if item.Streamed {
				streamedCount++
			}
		}
		if streamedCount != 1 {
			t.Errorf("streamed items = %d, want 1", streamedCount)
		}
	})
}

func mustLog(t *testing.T, id, tenant, model string, status ConversationStatus, createdAt time.Time) ConversationLog {
	t.Helper()
	log, err := NewConversationLog(id, tenant, model, status, []byte("request body"), createdAt)
	if err != nil {
		t.Fatalf("new conversation log: %v", err)
	}
	log.WithResponse([]byte("response body"))
	return *log
}

// memoryConversationStore is an in-memory test double for ConversationStore.
type memoryConversationStore struct {
	mu    sync.RWMutex
	items map[string]ConversationLog
}

func newMemoryConversationStore() *memoryConversationStore {
	return &memoryConversationStore{items: make(map[string]ConversationLog)}
}

func (m *memoryConversationStore) SaveBatch(_ context.Context, logs []ConversationLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range logs {
		if _, ok := m.items[l.ID]; !ok {
			m.items[l.ID] = l
		}
	}
	return nil
}

func (m *memoryConversationStore) DeleteOlderThan(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var deleted int64
	for id, l := range m.items {
		if l.CreatedAt.Before(before) {
			delete(m.items, id)
			deleted++
		}
	}
	return deleted, nil
}

func (m *memoryConversationStore) List(_ context.Context, filter ConversationFilter) (ConversationPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []ConversationLog
	for _, l := range m.items {
		if filter.TenantID != "" && l.TenantID != filter.TenantID {
			continue
		}
		all = append(all, l)
	}
	// deterministic order: newest first
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].CreatedAt.After(all[i].CreatedAt) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	perPage := filter.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * perPage
	if start > len(all) {
		start = len(all)
	}
	end := start + perPage
	if end > len(all) {
		end = len(all)
	}
	return ConversationPage{Items: all[start:end], Total: len(all)}, nil
}

func (m *memoryConversationStore) Get(_ context.Context, id string) (*ConversationLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := l
	return &copy, nil
}
