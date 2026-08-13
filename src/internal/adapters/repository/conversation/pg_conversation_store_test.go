package conversationrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
)

func getConversationTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to test DB: %v", err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS conversation_logs (
		id UUID PRIMARY KEY, tenant_id VARCHAR(255) NOT NULL,
		model VARCHAR(255) NOT NULL, status VARCHAR(32) NOT NULL,
		masked BOOLEAN NOT NULL DEFAULT false, streamed BOOLEAN NOT NULL DEFAULT false,
		mask_id VARCHAR(64),
		masking BYTEA,
		request BYTEA NOT NULL, response BYTEA,
		request_len INTEGER, response_len INTEGER,
		created_at TIMESTAMPTZ NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	_, err = pool.Exec(ctx, `DELETE FROM conversation_logs`)
	if err != nil {
		t.Fatalf("failed to truncate test table: %v", err)
	}
	return pool
}

func boolPtr(v bool) *bool { return &v }

func convLog(id, tenant, model string, status conversation.ConversationStatus, masked bool, createdAt time.Time) conversation.ConversationLog {
	l := conversation.ConversationLog{
		ID:         id,
		TenantID:   tenant,
		Model:      model,
		Status:     status,
		Masked:     masked,
		Request:    []byte("request body"),
		RequestLen: len("request body"),
		CreatedAt:  createdAt,
	}
	l.WithResponse([]byte("response body"))
	if masked {
		l.WithMasking([]byte("masking-ciphertext"))
	}
	return l
}

// @sk-test conversation-logging#T1.5: TestPgConversationStoreSaveBatchAndGet (AC-001, AC-005)
func TestPgConversationStoreSaveBatchAndGet(t *testing.T) {
	pool := getConversationTestPool(t)
	store := NewPgConversationStore(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	id := uuid.NewString()
	log := convLog(id, "tenant-1", "gpt-4o", conversation.StatusOK, true, now)

	if err := store.SaveBatch(ctx, []conversation.ConversationLog{log}); err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.TenantID != "tenant-1" || got.Model != "gpt-4o" {
		t.Errorf("unexpected record metadata: %+v", got)
	}
	if !got.Masked {
		t.Error("expected Masked=true")
	}
	if string(got.Request) != "request body" {
		t.Errorf("request = %q, want %q", got.Request, "request body")
	}
	if string(got.Response) != "response body" {
		t.Errorf("response = %q, want %q", got.Response, "response body")
	}
	if len(got.Masking) != len("masking-ciphertext") || string(got.Masking) != "masking-ciphertext" {
		t.Errorf("unexpected masking: %+v", got.Masking)
	}
}

// @sk-test conversation-logging#T5.1: TestPgConversationStoreMaskIDRoundTrip (AC-005)
func TestPgConversationStoreMaskIDRoundTrip(t *testing.T) {
	pool := getConversationTestPool(t)
	store := NewPgConversationStore(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	id := uuid.NewString()
	log := convLog(id, "tenant-1", "gpt-4o", conversation.StatusOK, true, now)
	log.WithMaskID("MASK_A1B2C3")

	if err := store.SaveBatch(ctx, []conversation.ConversationLog{log}); err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.MaskID != "MASK_A1B2C3" {
		t.Errorf("MaskID = %q, want MASK_A1B2C3", got.MaskID)
	}

	page, err := store.List(ctx, conversation.ConversationFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	for _, item := range page.Items {
		if item.ID == id && item.MaskID != "MASK_A1B2C3" {
			t.Errorf("list MaskID = %q, want MASK_A1B2C3", item.MaskID)
		}
	}
}

// @sk-test conversation-logging#T1.5: TestPgConversationStoreGetNotFound (AC-005)
func TestPgConversationStoreGetNotFound(t *testing.T) {
	pool := getConversationTestPool(t)
	store := NewPgConversationStore(pool)
	ctx := context.Background()

	if _, err := store.Get(ctx, uuid.NewString()); err != conversation.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// @sk-test conversation-logging#T1.5: TestPgConversationStoreListAndFilter (AC-006)
func TestPgConversationStoreListAndFilter(t *testing.T) {
	pool := getConversationTestPool(t)
	store := NewPgConversationStore(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	batch := []conversation.ConversationLog{
		convLog(uuid.NewString(), "tenant-1", "gpt-4o", conversation.StatusOK, false, now.Add(-time.Hour)),
		convLog(uuid.NewString(), "tenant-1", "claude", conversation.StatusBlocked, false, now.Add(-2*time.Hour)),
		convLog(uuid.NewString(), "tenant-2", "gpt-4o", conversation.StatusOK, false, now),
	}
	if err := store.SaveBatch(ctx, batch); err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}

	page, err := store.List(ctx, conversation.ConversationFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if page.Total != 3 {
		t.Errorf("Total = %d, want 3", page.Total)
	}
	for _, item := range page.Items {
		if len(item.Request) != 0 || item.Response != nil {
			t.Error("List must not return content payloads")
		}
	}

	tFiltered, err := store.List(ctx, conversation.ConversationFilter{TenantID: "tenant-1", Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List tenant filter failed: %v", err)
	}
	if tFiltered.Total != 2 {
		t.Errorf("tenant-filtered Total = %d, want 2", tFiltered.Total)
	}

	maskedLog := convLog(uuid.NewString(), "tenant-1", "gpt-4o", conversation.StatusOK, true, now.Add(-30*time.Minute))
	if err := store.SaveBatch(ctx, []conversation.ConversationLog{maskedLog}); err != nil {
		t.Fatalf("SaveBatch masked failed: %v", err)
	}

	maskedOnly, err := store.List(ctx, conversation.ConversationFilter{Masked: boolPtr(true), Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List masked=true failed: %v", err)
	}
	if maskedOnly.Total != 1 {
		t.Errorf("masked=true Total = %d, want 1", maskedOnly.Total)
	}
	for _, item := range maskedOnly.Items {
		if !item.Masked {
			t.Error("masked=true returned an unmasked record")
		}
	}

	unmaskedOnly, err := store.List(ctx, conversation.ConversationFilter{Masked: boolPtr(false), Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List masked=false failed: %v", err)
	}
	if unmaskedOnly.Total != 3 {
		t.Errorf("masked=false Total = %d, want 3", unmaskedOnly.Total)
	}
}

// @sk-test conversation-logging#T1.5: TestPgConversationStoreDeleteOlderThan (AC-007)
func TestPgConversationStoreDeleteOlderThan(t *testing.T) {
	pool := getConversationTestPool(t)
	store := NewPgConversationStore(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	batch := []conversation.ConversationLog{
		convLog(uuid.NewString(), "tenant-1", "gpt-4o", conversation.StatusOK, false, now.Add(-48*time.Hour)),
		convLog(uuid.NewString(), "tenant-1", "gpt-4o", conversation.StatusOK, false, now.Add(-time.Hour)),
	}
	if err := store.SaveBatch(ctx, batch); err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}

	deleted, err := store.DeleteOlderThan(ctx, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOlderThan failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	page, err := store.List(ctx, conversation.ConversationFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("Total after cleanup = %d, want 1", page.Total)
	}
}

// @sk-test conversation-logging#T2.4: TestPgConversationStoreStreamedRoundTrip (RQ-009, DEC-007, AC-003)
func TestPgConversationStoreStreamedRoundTrip(t *testing.T) {
	pool := getConversationTestPool(t)
	store := NewPgConversationStore(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	id := uuid.NewString()
	log := convLog(id, "tenant-1", "gpt-4o", conversation.StatusOK, false, now)
	log.Streamed = true

	if err := store.SaveBatch(ctx, []conversation.ConversationLog{log}); err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !got.Streamed {
		t.Error("expected Streamed=true after round-trip")
	}

	page, err := store.List(ctx, conversation.ConversationFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, item := range page.Items {
		if item.ID == id {
			found = true
			if !item.Streamed {
				t.Error("expected Streamed=true in list item")
			}
		}
	}
	if !found {
		t.Error("streamed record not found in list")
	}
}
