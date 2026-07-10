package mask

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
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

type mockDetector struct {
	results []detector.DetectorResult
}

func (d *mockDetector) Scan(_ context.Context, _ string) ([]detector.DetectorResult, error) {
	return d.results, nil
}

// @sk-test 22-shield-mask-storage#T2.2: Empty text returns empty replacements
func TestMaskText_EmptyText(t *testing.T) {
	store := newMemStorage()
	reg := detector.NewDetectorRegistry()
	uc := NewMaskUseCase(reg, store)

	masked, entry, err := uc.MaskText(context.Background(), "", "test-id")
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

// @sk-test 22-shield-mask-storage#T2.2: No detectors leaves text unchanged
func TestMaskText_NoDetectors(t *testing.T) {
	store := newMemStorage()
	reg := detector.NewDetectorRegistry()
	uc := NewMaskUseCase(reg, store)

	masked, entry, err := uc.MaskText(context.Background(), "hello world", "test-id")
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
	id := NewUUIDv7()
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
func TestMaskText_MaskIDConflict(t *testing.T) {
	store := newMemStorage()
	store.data["dup"] = &MaskEntry{MaskID: "dup", Replacements: map[string]string{}}
	reg := detector.NewDetectorRegistry()
	uc := NewMaskUseCase(reg, store)

	_, _, err := uc.MaskText(context.Background(), "hello", "dup")
	if !errors.Is(err, ErrMaskIDConflict) {
		t.Errorf("expected ErrMaskIDConflict, got %v", err)
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Single detector replacement with placeholder
func TestMaskText_SingleReplacement(t *testing.T) {
	store := newMemStorage()
	reg := detector.NewDetectorRegistry()
	err := reg.Register(entity.DetectorTypeRegex, &mockDetector{
		results: []detector.DetectorResult{
			{DetectorType: "email", Fragment: "test@example.com", StartPos: 3, EndPos: 19, Confidence: 1.0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	uc := NewMaskUseCase(reg, store)

	masked, entry, err := uc.MaskText(context.Background(), "Hi test@example.com!", "abc")
	if err != nil {
		t.Fatal(err)
	}
	expected := "Hi {{abc.1}}!"
	if masked != expected {
		t.Errorf("expected %q, got %q", expected, masked)
	}
	if entry.Replacements["{{abc.1}}"] != "test@example.com" {
		t.Errorf("expected replacement for {{abc.1}}, got %v", entry.Replacements)
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Unmask restores single placeholder
func TestUnmaskText_Single(t *testing.T) {
	store := newMemStorage()
	store.data["abc"] = &MaskEntry{
		MaskID: "abc",
		Replacements: map[string]string{
			"{{abc.1}}": "test@example.com",
		},
	}
	reg := detector.NewDetectorRegistry()
	uc := NewMaskUseCase(reg, store)

	restored, err := uc.UnmaskText(context.Background(), "Hi {{abc.1}}!", []string{"abc"})
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
	reg := detector.NewDetectorRegistry()
	err := reg.Register(entity.DetectorTypeRegex, &mockDetector{
		results: []detector.DetectorResult{
			{DetectorType: "email", Fragment: "john@example.com", StartPos: 9, EndPos: 25, Confidence: 1.0},
			{DetectorType: "phone", Fragment: "+1-555-1234", StartPos: 34, EndPos: 45, Confidence: 1.0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	uc := NewMaskUseCase(reg, store)
	original := "Contact: john@example.com, Phone: +1-555-1234"

	masked, _, err := uc.MaskText(context.Background(), original, "rt")
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
		MaskID: "a",
		Replacements: map[string]string{
			"{{a.1}}": "alice@example.com",
		},
	}
	store.data["b"] = &MaskEntry{
		MaskID: "b",
		Replacements: map[string]string{
			"{{b.1}}": "+1-555-0000",
		},
	}
	reg := detector.NewDetectorRegistry()
	uc := NewMaskUseCase(reg, store)

	restored, err := uc.UnmaskText(context.Background(),
		"Email: {{a.1}}, Phone: {{b.1}}", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	expected := "Email: alice@example.com, Phone: +1-555-0000"
	if restored != expected {
		t.Errorf("expected %q, got %q", expected, restored)
	}
}

// @sk-test 22-shield-mask-storage#T2.2: Overlapping results filtered, longer wins
func TestMaskText_OverlapFilter(t *testing.T) {
	store := newMemStorage()
	reg := detector.NewDetectorRegistry()
	err := reg.Register(entity.DetectorTypeRegex, &mockDetector{
		results: []detector.DetectorResult{
			{DetectorType: "email", Fragment: "john@example.com", StartPos: 0, EndPos: 16, Confidence: 1.0},
			{DetectorType: "domain", Fragment: "example.com", StartPos: 5, EndPos: 16, Confidence: 1.0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	uc := NewMaskUseCase(reg, store)

	masked, entry, err := uc.MaskText(context.Background(), "john@example.com", "ov")
	if err != nil {
		t.Fatal(err)
	}
	if masked != "{{ov.1}}" {
		t.Errorf("expected %q, got %q", "{{ov.1}}", masked)
	}
	if len(entry.Replacements) != 1 {
		t.Errorf("expected 1 replacement (deduped), got %d", len(entry.Replacements))
	}
}
