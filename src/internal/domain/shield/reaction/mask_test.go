package reaction

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type memMaskStorage struct {
	mu   sync.RWMutex
	data map[string]*mask.MaskEntry
}

func newMemMaskStorage() *memMaskStorage {
	return &memMaskStorage{data: make(map[string]*mask.MaskEntry)}
}

func (s *memMaskStorage) Save(_ context.Context, entry *mask.MaskEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *entry
	s.data[entry.MaskID] = &cp
	return nil
}

func (s *memMaskStorage) Get(_ context.Context, maskID string) (*mask.MaskEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[maskID]
	if !ok {
		return nil, mask.ErrMaskNotFound
	}
	cp := *e
	return &cp, nil
}

func (s *memMaskStorage) Delete(_ context.Context, maskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, maskID)
	return nil
}

// @sk-test 23-shield-reactions#T3.1: Test MaskReaction replaces fragments with placeholders (AC-003)
func TestMaskReaction_ReplacesWithPlaceholder(t *testing.T) {
	store := newMemMaskStorage()
	uc := mask.NewMaskUseCase(nil, store)
	mr := NewMaskReaction(uc)

	incidents := []entity.Incident{
		*entity.NewIncident("email", mustPatternID("p1"), value.SeverityCritical, "user@example.com", 6),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := mr.Execute(context.Background(), result, "email: user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "{{") || !strings.Contains(out, "}}") {
		t.Errorf("expected placeholder in output, got %q", out)
	}
	if strings.Contains(out, "user@example.com") {
		t.Errorf("expected sensitive data to be masked, got %q", out)
	}
}

// @sk-test 23-shield-reactions#T3.1: Test MaskReaction with nil result
func TestMaskReaction_NilResult(t *testing.T) {
	store := newMemMaskStorage()
	uc := mask.NewMaskUseCase(nil, store)
	mr := NewMaskReaction(uc)

	out, err := mr.Execute(context.Background(), nil, "text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "text" {
		t.Errorf("expected original text, got %q", out)
	}
}

// @sk-test 23-shield-reactions#T3.1: Test MaskReaction with empty incidents
func TestMaskReaction_EmptyIncidents(t *testing.T) {
	store := newMemMaskStorage()
	uc := mask.NewMaskUseCase(nil, store)
	mr := NewMaskReaction(uc)

	result := entity.NewScanResult(value.ScanStatusClean, nil)

	out, err := mr.Execute(context.Background(), result, "text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "text" {
		t.Errorf("expected original text, got %q", out)
	}
}

// @sk-test 23-shield-reactions#T3.1: Test MaskReaction searches fragment when position is 0 (AC-003)
func TestMaskReaction_SearchesFragmentWhenPositionZero(t *testing.T) {
	store := newMemMaskStorage()
	uc := mask.NewMaskUseCase(nil, store)
	mr := NewMaskReaction(uc)

	incidents := []entity.Incident{
		*entity.NewIncident("email", mustPatternID("p1"), value.SeverityCritical, "user@example.com", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := mr.Execute(context.Background(), result, "Contact: user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "{{") {
		t.Errorf("expected placeholder in output, got %q", out)
	}
}

// @sk-test 23-shield-reactions#T3.1: Test MaskReaction skips fragment not found in text
func TestMaskReaction_FragmentNotFound(t *testing.T) {
	store := newMemMaskStorage()
	uc := mask.NewMaskUseCase(nil, store)
	mr := NewMaskReaction(uc)

	incidents := []entity.Incident{
		*entity.NewIncident("det", mustPatternID("p1"), value.SeverityHigh, "nonexistent", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := mr.Execute(context.Background(), result, "some text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "some text" {
		t.Errorf("expected original text, got %q", out)
	}
}
