package session

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type mockStore struct {
	sessions map[string]*Session
}

func newMockStore() *mockStore {
	return &mockStore{sessions: make(map[string]*Session)}
}

func (m *mockStore) Save(ctx context.Context, s *Session) error {
	if _, exists := m.sessions[s.SessionID]; exists {
		return ErrSessionConflict
	}
	m.sessions[s.SessionID] = s
	return nil
}

func (m *mockStore) Get(ctx context.Context, tenantID, sessionID string) (*Session, error) {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return nil, ErrSessionNotFound
	}
	return s, nil
}

func (m *mockStore) IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return ErrSessionNotFound
	}
	s.TokenCount += tokens
	s.MessageCount += messages
	s.TotalMasks += totalMasks
	s.DictMaskCount += dictMaskCount
	s.PIIMaskCount += piiMaskCount
	s.PreprocessorCount += preprocessorCount
	return nil
}

func (m *mockStore) ExtendTTL(ctx context.Context, tenantID, sessionID string, newExpiresAt time.Time) error {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return ErrSessionNotFound
	}
	s.ExpiresAt = newExpiresAt
	return nil
}

func (m *mockStore) Close(ctx context.Context, tenantID, sessionID string) error {
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return ErrSessionNotFound
	}
	s.Status = SessionStatusClosed
	return nil
}

func (m *mockStore) DeleteExpired(ctx context.Context) (int64, error) {
	var deleted int64
	for id, s := range m.sessions {
		if s.Status == SessionStatusExpired || time.Now().After(s.ExpiresAt) {
			delete(m.sessions, id)
			deleted++
		}
	}
	return deleted, nil
}

func (m *mockStore) ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*ListResult, error) {
	var items []Session
	for _, s := range m.sessions {
		if s.TenantID == tenantID {
			items = append(items, *s)
		}
	}
	total := len(items)
	if items == nil {
		items = []Session{}
	}
	return &ListResult{Items: items, Total: total, Page: int(page), Limit: int(limit)}, nil
}

func (m *mockStore) ListAll(ctx context.Context, page, limit int32) (*ListResult, error) {
	var items []Session
	for _, s := range m.sessions {
		items = append(items, *s)
	}
	total := len(items)
	if items == nil {
		items = []Session{}
	}
	return &ListResult{Items: items, Total: total, Page: int(page), Limit: int(limit)}, nil
}

// @sk-test sessions#T2.4: TestCreateSession (AC-001)
func TestCreateSession(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	sess, err := uc.Create(context.Background(), "test-id", "tenant-alpha", "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.SessionID != "test-id" {
		t.Errorf("expected test-id, got %s", sess.SessionID)
	}
	if sess.TenantID != "tenant-alpha" {
		t.Errorf("expected tenant-alpha, got %s", sess.TenantID)
	}
	if sess.Status != SessionStatusActive {
		t.Errorf("expected active, got %s", sess.Status)
	}
	if sess.ExpiresAt.Before(sess.CreatedAt) {
		t.Errorf("expires_at should be after created_at")
	}
}

// @sk-test sessions#T2.4: TestCreateSessionConflict (AC-001)
func TestCreateSessionConflict(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "dup-id", "tenant-alpha", "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = uc.Create(context.Background(), "dup-id", "tenant-alpha", "gpt-4", 30*time.Minute)
	if !errors.Is(err, ErrSessionConflict) {
		t.Errorf("expected ErrSessionConflict, got %v", err)
	}
}

// @sk-test sessions#T2.4: TestCreateSessionEmptyTenant (AC-001)
func TestCreateSessionEmptyTenant(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "id", "", "gpt-4", 30*time.Minute)
	if err == nil {
		t.Fatal("expected error for empty tenant_id")
	}
}

// @sk-test sessions#T2.4: TestGetSessionTenantScoped (AC-003)
func TestGetSessionTenantScoped(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "sess-a", "tenant-alpha", "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = uc.Get(context.Background(), "tenant-beta", "sess-a")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound for wrong tenant, got %v", err)
	}
}

// @sk-test sessions#T2.4: TestIncrementCounts (AC-002)
func TestIncrementCounts(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "sess", "tenant-alpha", "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = uc.IncrementCounts(context.Background(), "tenant-alpha", "sess", 100, 1, 3, 2, 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, _ := uc.Get(context.Background(), "tenant-alpha", "sess")
	if sess.TokenCount != 100 || sess.MessageCount != 1 || sess.TotalMasks != 3 || sess.DictMaskCount != 2 || sess.PIIMaskCount != 1 || sess.PreprocessorCount != 0 {
		t.Errorf("unexpected counts: token=%d msg=%d masks=%d dict=%d pii=%d prep=%d",
			sess.TokenCount, sess.MessageCount, sess.TotalMasks, sess.DictMaskCount, sess.PIIMaskCount, sess.PreprocessorCount)
	}
}

// @sk-test sessions#T2.4: TestExtendTTL (AC-005)
func TestExtendTTL(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "sess", "tenant-alpha", "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := uc.ExtendTTL(context.Background(), "tenant-alpha", "sess", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ExpiresAt.Before(time.Now().Add(30 * time.Minute)) {
		t.Errorf("expires_at should be extended")
	}
}

// @sk-test sessions#T2.4: TestCloseSession (AC-006)
func TestCloseSession(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "sess", "tenant-alpha", "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = uc.Close(context.Background(), "tenant-alpha", "sess")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, _ := uc.Get(context.Background(), "tenant-alpha", "sess")
	if sess.Status != SessionStatusClosed {
		t.Errorf("expected closed, got %s", sess.Status)
	}
}

// @sk-test sessions#T2.4: TestCloseSessionTwice (AC-006)
func TestCloseSessionTwice(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "sess", "tenant-alpha", "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = uc.Close(context.Background(), "tenant-alpha", "sess")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = uc.Close(context.Background(), "tenant-alpha", "sess")
	if !errors.Is(err, ErrSessionClosed) {
		t.Errorf("expected ErrSessionClosed, got %v", err)
	}
}

// @sk-test sessions#T2.4: TestCloseSessionWrongTenant (AC-003)
func TestCloseSessionWrongTenant(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "sess", "tenant-alpha", "gpt-4", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = uc.Close(context.Background(), "tenant-beta", "sess")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// @sk-test sessions#T2.4: TestListByTenant (AC-004)
func TestListByTenant(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess-%d", i)
		_, err := uc.Create(context.Background(), id, "tenant-alpha", "gpt-4", 30*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	result, err := uc.ListByTenant(context.Background(), "tenant-alpha", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Total)
	}
	if len(result.Items) != 5 {
		t.Errorf("expected 5 items, got %d", len(result.Items))
	}
}

// @sk-test sessions#T2.4: TestDeleteExpired (AC-007)
func TestDeleteExpired(t *testing.T) {
	store := newMockStore()
	uc := NewSessionUseCase(store)

	_, err := uc.Create(context.Background(), "sess-1", "tenant-alpha", "gpt-4", -1*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deleted, err := uc.DeleteExpired(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted == 0 {
		t.Errorf("expected at least 1 deleted, got %d", deleted)
	}
}
