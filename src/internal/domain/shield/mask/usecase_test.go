package mask

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
)

type memStorage struct {
	mu   sync.RWMutex
	data map[string]*MaskEntry
}

func newMemStorage() *memStorage {
	return &memStorage{data: make(map[string]*MaskEntry)}
}

func (s *memStorage) Save(_ context.Context, entry *MaskEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[entry.MaskID]; ok {
		return ErrMaskIDConflict
	}
	cp := *entry
	s.data[entry.MaskID] = &cp
	return nil
}

func (s *memStorage) Get(_ context.Context, maskID string) (*MaskEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[maskID]
	if !ok {
		return nil, ErrMaskNotFound
	}
	cp := *e
	return &cp, nil
}

func (s *memStorage) Delete(_ context.Context, maskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, maskID)
	return nil
}

// @sk-test 22-shield-mask-storage#T2.2: Empty results returns empty replacements
func TestMaskFromResults_EmptyResults(t *testing.T) {
	store := newMemStorage()
	uc := NewMaskUseCase(nil, store)

	masked, entry, err := uc.MaskFromResults(context.Background(), "", "test-id", "test-id", nil)
	if err != nil {
		t.Fatal(err)
	}
	if masked != "" {
		t.Errorf("expected empty, got %q", masked)
	}
	if entry == nil || len(entry.Replacements) != 0 {
		t.Errorf("expected empty replacements map")
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Empty results leaves text unchanged
func TestMaskFromResults_NoResults(t *testing.T) {
	store := newMemStorage()
	uc := NewMaskUseCase(nil, store)

	masked, entry, err := uc.MaskFromResults(context.Background(), "hello world", "test-id", "test-id", nil)
	if err != nil {
		t.Fatal(err)
	}
	if masked != "hello world" {
		t.Errorf("expected original text, got %q", masked)
	}
	if entry == nil || len(entry.Replacements) != 0 {
		t.Errorf("expected empty replacements")
	}
}

// @sk-test 22-shield-mask-storage#T2.2: UUIDv7 format, version and variant
func TestNewUUIDv7_Format(t *testing.T) {
	id, err := NewUUIDv7()
	if err != nil {
		t.Fatalf("NewUUIDv7 failed: %v", err)
	}
	if len(id) != 36 {
		t.Errorf("expected 36 chars, got %d: %q", len(id), id)
	}
	if id[14] != '7' {
		t.Errorf("expected version 7 at position 14, got %c", id[14])
	}
	if id[19] != '8' && id[19] != '9' && id[19] != 'a' && id[19] != 'b' {
		t.Errorf("expected variant 10xx at position 19, got %c", id[19])
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Duplicate mask_id returns conflict
func TestMaskFromResults_MaskIDConflict(t *testing.T) {
	store := newMemStorage()
	store.data["dup"] = &MaskEntry{MaskID: "dup", DocumentMaskID: "dup", Replacements: map[string]string{}}
	uc := NewMaskUseCase(nil, store)

	_, _, err := uc.MaskFromResults(context.Background(), "hello", "dup", "dup", nil)
	if !errors.Is(err, ErrMaskIDConflict) {
		t.Errorf("expected ErrMaskIDConflict, got %v", err)
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Single detector replacement with placeholder
func TestMaskFromResults_SingleReplacement(t *testing.T) {
	store := newMemStorage()
	uc := NewMaskUseCase(nil, store)

	masked, entry, err := uc.MaskFromResults(context.Background(), "Hi test@example.com!", "abc", "abc",
		[]detector.DetectorResult{
			{DetectorType: "email", Fragment: "test@example.com", StartPos: 3, EndPos: 19, Confidence: 1.0},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Hi [MASK_abc.1]!"
	if masked != expected {
		t.Errorf("expected %q, got %q", expected, masked)
	}
	if entry.Replacements["[MASK_abc.1]"] != "test@example.com" {
		t.Errorf("expected replacement for [MASK_abc.1], got %v", entry.Replacements)
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Unmask restores single placeholder
func TestUnmaskText_Single(t *testing.T) {
	store := newMemStorage()
	store.data["abc"] = &MaskEntry{
		MaskID:         "abc",
		DocumentMaskID: "abc",
		Replacements: map[string]string{
			"[MASK_abc.1]": "test@example.com",
		},
	}
	reg := detector.NewDetectorRegistry()
	uc := NewMaskUseCase(reg, store)

	restored, err := uc.UnmaskText(context.Background(), "Hi [MASK_abc.1]!", []string{"abc"})
	if err != nil {
		t.Fatal(err)
	}
	if restored != "Hi test@example.com!" {
		t.Errorf("expected %q, got %q", "Hi test@example.com!", restored)
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Mask then unmask returns original text
func TestMaskUnmask_RoundTrip(t *testing.T) {
	store := newMemStorage()
	uc := NewMaskUseCase(nil, store)
	original := "Contact: john@example.com, Phone: +1-555-1234"

	masked, _, err := uc.MaskFromResults(context.Background(), original, "rt", "rt",
		[]detector.DetectorResult{
			{DetectorType: "email", Fragment: "john@example.com", StartPos: 9, EndPos: 25, Confidence: 1.0},
			{DetectorType: "phone", Fragment: "+1-555-1234", StartPos: 34, EndPos: 45, Confidence: 1.0},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	restored, err := uc.UnmaskText(context.Background(), masked, []string{"rt"})
	if err != nil {
		t.Fatal(err)
	}
	if restored != original {
		t.Errorf("round-trip: expected %q, got %q", original, restored)
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Multiple mask_ids unmask merged
func TestMaskText_MultipleMaskIDs(t *testing.T) {
	store := newMemStorage()
	store.data["a"] = &MaskEntry{
		MaskID:         "a",
		DocumentMaskID: "a",
		Replacements: map[string]string{
			"[MASK_a.1]": "alice@example.com",
		},
	}
	store.data["b"] = &MaskEntry{
		MaskID:         "b",
		DocumentMaskID: "b",
		Replacements: map[string]string{
			"[MASK_b.1]": "+1-555-0000",
		},
	}
	reg := detector.NewDetectorRegistry()
	uc := NewMaskUseCase(reg, store)

	restored, err := uc.UnmaskText(context.Background(),
		"Email: [MASK_a.1], Phone: [MASK_b.1]", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	expected := "Email: alice@example.com, Phone: +1-555-0000"
	if restored != expected {
		t.Errorf("expected %q, got %q", expected, restored)
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Overlapping results filtered, longer wins
func TestMaskFromResults_OverlapFilter(t *testing.T) {
	store := newMemStorage()
	uc := NewMaskUseCase(nil, store)

	masked, entry, err := uc.MaskFromResults(context.Background(), "john@example.com", "ov", "ov",
		[]detector.DetectorResult{
			{DetectorType: "email", Fragment: "john@example.com", StartPos: 0, EndPos: 16, Confidence: 1.0},
			{DetectorType: "domain", Fragment: "example.com", StartPos: 5, EndPos: 16, Confidence: 1.0},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if masked != "[MASK_ov.1]" {
		t.Errorf("expected %q, got %q", "[MASK_ov.1]", masked)
	}
	if len(entry.Replacements) != 1 {
		t.Errorf("expected 1 replacement (deduped), got %d", len(entry.Replacements))
	}
}
