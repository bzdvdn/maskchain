package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
)

// maxContentSize caps each captured payload before encryption to bound DB growth.
const maxContentSize = 1 << 20 // 1MB

// @sk-task conversation-logging#T2.1: Implement RawRecord type carrying plaintext capture (RQ-008, AC-011)
// @sk-task conversation-logging#T2.4: Add Streamed flag to RawRecord (RQ-009, DEC-007, AC-003)
// @sk-task conversation-logging#T5.1: Carry MaskID linking the log to the shield dict mask (AC-005)
//
// RawRecord carries the unencrypted capture from the hot path to the worker.
type RawRecord struct {
	ID        string
	TenantID  string
	Model     string
	Status    conversation.ConversationStatus
	Streamed  bool
	MaskID    string
	Detector  string
	Category  string
	Request   []byte
	Response  []byte
	Masking   []conversation.MaskingEntry
	CreatedAt time.Time
}

// @sk-task conversation-logging#T2.2: Implement Sender interface decoupling middleware from worker (RQ-008, DEC-002)
//
// Sender is the minimal contract the conversation middleware depends on.
type Sender interface {
	Send(rec RawRecord)
}

// @sk-task conversation-logging#T2.1: Implement AsyncWorker encrypting off the hot path (RQ-008, AC-011)
//
// AsyncWorker receives raw records over a buffered channel, encrypts all
// content fields in its own goroutine, and batch-inserts ciphertext.
type AsyncWorker struct {
	store    conversation.ConversationStore
	enc      *crypto.Encryptor
	buffer   chan RawRecord
	interval time.Duration
	log      *slog.Logger
}

// @sk-task conversation-logging#T2.1: NewAsyncWorker creates a new AsyncWorker (AC-011)
func NewAsyncWorker(store conversation.ConversationStore, enc *crypto.Encryptor, bufferSize int, interval time.Duration, log *slog.Logger) *AsyncWorker {
	return &AsyncWorker{
		store:    store,
		enc:      enc,
		buffer:   make(chan RawRecord, bufferSize),
		interval: interval,
		log:      log,
	}
}

// @sk-task conversation-logging#T2.1: Run encrypts and batch-persists records (RQ-008, AC-011)
func (w *AsyncWorker) Run(ctx context.Context) {
	w.log.Info("conversation async worker started", slog.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	var batch []conversation.ConversationLog

	flush := func() {
		if len(batch) == 0 {
			return
		}
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := w.store.SaveBatch(flushCtx, batch); err != nil {
			w.log.Warn("conversation async worker: batch insert failed",
				slog.String("error", err.Error()), slog.Int("size", len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			w.log.Info("conversation async worker stopped")
			return
		case <-ticker.C:
			flush()
		case rec := <-w.buffer:
			log, err := w.encrypt(rec)
			if err != nil {
				w.log.Warn("conversation async worker: encrypt failed",
					slog.String("error", err.Error()), slog.String("trace_id", rec.ID))
				continue
			}
			batch = append(batch, *log)
			if len(batch) >= cap(w.buffer) {
				flush()
			}
		}
	}
}

// @sk-task conversation-logging#T2.1: encrypt builds an encrypted ConversationLog (AC-004, AC-011)
// @sk-task conversation-logging#T2.4: Carry Streamed flag through encryption (RQ-009, DEC-007, AC-003)
func (w *AsyncWorker) encrypt(rec RawRecord) (*conversation.ConversationLog, error) {
	var log *conversation.ConversationLog
	var err error
	if len(rec.Request) == 0 {
		// Metadata-only capture (retention mode `meta`): no content is ever
		// encrypted or stored; only block facts ride through (DEC-003, AC-002).
		log, err = conversation.NewMetadataOnlyConversationLog(rec.ID, rec.TenantID, rec.Model, rec.Status, rec.CreatedAt)
		if err != nil {
			return nil, err
		}
		log.Streamed = rec.Streamed
		log.WithMaskID(rec.MaskID)
		log.Detector = rec.Detector
		log.Category = rec.Category
		return log, nil
	}

	encReq, err := w.enc.Encrypt(trimToMax(rec.Request))
	if err != nil {
		return nil, fmt.Errorf("encrypt request: %w", err)
	}
	var encResp []byte
	if len(rec.Response) > 0 {
		encResp, err = w.enc.Encrypt(trimToMax(rec.Response))
		if err != nil {
			return nil, fmt.Errorf("encrypt response: %w", err)
		}
	}
	var encMasking []byte
	if len(rec.Masking) > 0 {
		plain, err := conversation.MaskingEntriesJSON(rec.Masking)
		if err != nil {
			return nil, fmt.Errorf("serialize masking: %w", err)
		}
		encMasking, err = w.enc.Encrypt(trimToMax(plain))
		if err != nil {
			return nil, fmt.Errorf("encrypt masking: %w", err)
		}
	}

	log, err = conversation.NewConversationLog(rec.ID, rec.TenantID, rec.Model, rec.Status, encReq, rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	log.Streamed = rec.Streamed
	log.WithResponse(encResp)
	log.WithMasking(encMasking)
	log.WithMaskID(rec.MaskID)
	log.Detector = rec.Detector
	log.Category = rec.Category
	log.RequestLen = len(rec.Request)
	log.ResponseLen = len(rec.Response)
	return log, nil
}

// @sk-task conversation-logging#T2.1: Send enqueues a raw record non-blocking (AC-011)
func (w *AsyncWorker) Send(rec RawRecord) {
	select {
	case w.buffer <- rec:
	default:
		w.log.Warn("conversation async worker: buffer full, dropping record")
	}
}

// @sk-task conversation-logging#T2.1: Buffer returns the send-only channel for direct use (AC-011)
func (w *AsyncWorker) Buffer() chan<- RawRecord {
	return w.buffer
}

func trimToMax(data []byte) []byte {
	if len(data) > maxContentSize {
		return data[:maxContentSize]
	}
	return data
}
