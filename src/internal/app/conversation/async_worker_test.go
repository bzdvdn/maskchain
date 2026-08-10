package conversation

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
)

const testKeyB64 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func newTestEncryptor(t *testing.T) *crypto.Encryptor {
	t.Helper()
	enc, err := crypto.New(testKeyB64)
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return enc
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

type mockConversationStore struct {
	mu          sync.Mutex
	saveBatches [][]conversation.ConversationLog
	deletedAt   []time.Time
}

func (m *mockConversationStore) SaveBatch(_ context.Context, logs []conversation.ConversationLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	batch := make([]conversation.ConversationLog, len(logs))
	copy(batch, logs)
	m.saveBatches = append(m.saveBatches, batch)
	return nil
}

func (m *mockConversationStore) DeleteOlderThan(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedAt = append(m.deletedAt, before)
	return 0, nil
}

func (m *mockConversationStore) List(_ context.Context, _ conversation.ConversationFilter) (conversation.ConversationPage, error) {
	return conversation.ConversationPage{}, nil
}

func (m *mockConversationStore) Get(_ context.Context, _ string) (*conversation.ConversationLog, error) {
	return nil, conversation.ErrNotFound
}

func testRawRecord(i int) RawRecord {
	return RawRecord{
		ID:        "rec-" + string(rune('0'+i)),
		TenantID:  "tenant-1",
		Model:     "gpt-4o",
		Status:    conversation.StatusOK,
		MaskID:    "MASK_A1B2C3",
		Request:   []byte("original request"),
		Response:  []byte("masked response"),
		Masking:   []conversation.MaskingEntry{{Placeholder: "[MASK_1]", Original: "secret@example.com"}},
		CreatedAt: time.Now().UTC(),
	}
}

// @sk-test conversation-logging#T2.1: TestAsyncWorkerBatchInsert (AC-011)
func TestAsyncWorkerBatchInsert(t *testing.T) {
	store := &mockConversationStore{}
	worker := NewAsyncWorker(store, newTestEncryptor(t), 1000, 50*time.Millisecond, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go worker.Run(ctx)

	for i := 0; i < 10; i++ {
		worker.Send(testRawRecord(i))
	}

	<-ctx.Done()

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.saveBatches) == 0 {
		t.Fatal("expected at least 1 SaveBatch call")
	}

	var totalRecords int
	for _, b := range store.saveBatches {
		totalRecords += len(b)
	}
	if totalRecords < 10 {
		t.Errorf("expected at least 10 records in batch(es), got %d", totalRecords)
	}

	last := store.saveBatches[len(store.saveBatches)-1]
	if len(last) == 0 {
		t.Fatal("expected a non-empty batch")
	}
	enc := newTestEncryptor(t)
	decReq, err := enc.Decrypt(last[0].Request)
	if err != nil {
		t.Fatalf("decrypt request: %v", err)
	}
	if string(decReq) != "original request" {
		t.Errorf("decrypted request = %q, want %q", decReq, "original request")
	}
	if !last[0].Masked {
		t.Error("expected Masked=true when masking present")
	}
	decMasking, err := enc.Decrypt(last[0].Masking)
	if err != nil {
		t.Fatalf("decrypt masking: %v", err)
	}
	if string(decMasking) != `[{"placeholder":"[MASK_1]","original":"secret@example.com"}]` {
		t.Errorf("decrypted masking = %q", decMasking)
	}
	if last[0].MaskID != "MASK_A1B2C3" {
		t.Errorf("MaskID = %q, want MASK_A1B2C3", last[0].MaskID)
	}
}

// @sk-task conversation-logging#T2.1: TestAsyncWorkerGracefulShutdown (AC-011)
func TestAsyncWorkerGracefulShutdown(t *testing.T) {
	store := &mockConversationStore{}
	worker := NewAsyncWorker(store, newTestEncryptor(t), 1000, time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	worker.Send(testRawRecord(1))
	worker.Send(testRawRecord(2))

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.saveBatches) != 1 {
		t.Errorf("expected 1 SaveBatch call on shutdown, got %d", len(store.saveBatches))
	}
}

// @sk-test conversation-logging#T2.1: TestAsyncWorkerBufferOverflow (AC-011)
func TestAsyncWorkerBufferOverflow(t *testing.T) {
	store := &mockConversationStore{}
	worker := NewAsyncWorker(store, newTestEncryptor(t), 5, time.Hour, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go worker.Run(ctx)

	for i := 0; i < 10; i++ {
		worker.Send(testRawRecord(i))
	}

	<-ctx.Done()

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.saveBatches) == 0 {
		t.Error("expected SaveBatch to be called")
	}
}
