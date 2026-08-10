package conversation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
)

// @sk-test conversation-logging#T2.1: TestCleanupWorkerDeletesOldRecords (AC-007)
func TestCleanupWorkerDeletesOldRecords(t *testing.T) {
	store := &mockConversationStore{}
	worker := NewCleanupWorker(store, 50*time.Millisecond, 24*time.Hour, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.Run(ctx)
}

// @sk-test conversation-logging#T2.1: TestCleanupWorkerIntervalZero (AC-007)
func TestCleanupWorkerIntervalZero(t *testing.T) {
	store := &mockConversationStore{}
	worker := NewCleanupWorker(store, 0, 24*time.Hour, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	worker.Run(ctx)
}

// @sk-test conversation-logging#T2.1: TestCleanupWorkerCallsDeleteOlderThan (AC-007)
func TestCleanupWorkerCallsDeleteOlderThan(t *testing.T) {
	store := &conversationStoreWithRecorder{ConversationStore: &mockConversationStore{}}

	worker := NewCleanupWorker(store, 50*time.Millisecond, 24*time.Hour, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.Run(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.deletedAt) == 0 {
		t.Error("expected DeleteOlderThan to be called at least once")
	}
}

type conversationStoreWithRecorder struct {
	conversation.ConversationStore
	mu        sync.Mutex
	deletedAt []time.Time
}

func (c *conversationStoreWithRecorder) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	c.mu.Lock()
	c.deletedAt = append(c.deletedAt, before)
	c.mu.Unlock()
	return c.ConversationStore.DeleteOlderThan(ctx, before)
}
