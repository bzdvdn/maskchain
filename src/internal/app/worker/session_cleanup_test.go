package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/session"
)

type mockStore struct {
	sessions map[string]*session.Session
	mu       sync.Mutex
}

func newMockStore() *mockStore {
	return &mockStore{sessions: make(map[string]*session.Session)}
}

func (m *mockStore) Save(ctx context.Context, s *session.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.SessionID] = s
	return nil
}

func (m *mockStore) Get(ctx context.Context, tenantID, sessionID string) (*session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, exists := m.sessions[sessionID]
	if !exists || s.TenantID != tenantID {
		return nil, session.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockStore) IncrementCounts(ctx context.Context, tenantID, sessionID string, tokens int64, messages int32, totalMasks, dictMaskCount, piiMaskCount, preprocessorCount int32) error {
	return nil
}

func (m *mockStore) ExtendTTL(ctx context.Context, tenantID, sessionID string, newExpiresAt time.Time) error {
	return session.ErrSessionNotFound
}

func (m *mockStore) Close(ctx context.Context, tenantID, sessionID string) error {
	return session.ErrSessionNotFound
}

func (m *mockStore) DeleteExpired(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var deleted int64
	for id, s := range m.sessions {
		if s.Status == session.SessionStatusExpired || time.Now().After(s.ExpiresAt) {
			delete(m.sessions, id)
			deleted++
		}
	}
	return deleted, nil
}

func (m *mockStore) ListByTenant(ctx context.Context, tenantID string, page, limit int32) (*session.ListResult, error) {
	return &session.ListResult{Items: []session.Session{}, Total: 0, Page: 1, Limit: 20}, nil
}

func (m *mockStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// @sk-test sessions#T5.3: TestCleanupWorkerDeletesExpiredSessions (AC-007)
func TestCleanupWorkerDeletesExpiredSessions(t *testing.T) {
	store := newMockStore()
	uc := session.NewSessionUseCase(store)

	now := time.Now()
	if err := store.Save(context.Background(), &session.Session{
		SessionID: "expired-1",
		TenantID:  "t1",
		Status:    session.SessionStatusExpired,
		ExpiresAt: now.Add(-1 * time.Hour),
		CreatedAt: now.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("save expired session: %v", err)
	}
	if err := store.Save(context.Background(), &session.Session{
		SessionID: "active-1",
		TenantID:  "t1",
		Status:    session.SessionStatusActive,
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("save active session: %v", err)
	}

	if store.count() != 2 {
		t.Fatalf("expected 2 sessions, got %d", store.count())
	}

	worker := NewCleanupWorker(uc, 10*time.Millisecond, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	worker.Run(ctx)

	if store.count() != 1 {
		t.Errorf("expected 1 session after cleanup (expired removed), got %d", store.count())
	}

	_, err := store.Get(context.Background(), "t1", "active-1")
	if err != nil {
		t.Errorf("active session should still exist, got %v", err)
	}
}

// @sk-test sessions#T5.3: TestCleanupWorkerIntervalZero (AC-007)
func TestCleanupWorkerIntervalZero(t *testing.T) {
	store := newMockStore()
	uc := session.NewSessionUseCase(store)

	worker := NewCleanupWorker(uc, 0, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	worker.Run(ctx)
}
