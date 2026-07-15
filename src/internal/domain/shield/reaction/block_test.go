package reaction

import (
	"context"
	"errors"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

func mustPatternID(v string) value.PatternID {
	id, err := value.NewPatternID(v)
	if err != nil {
		panic(err)
	}
	return id
}

// @sk-test remove-audit-incidents#T4.2: Test BlockReaction returns ErrBlockedByPolicy (AC-001)
func TestBlockReaction_ReturnsBlockedError(t *testing.T) {
	r := NewBlockReaction()
	result := entity.NewScanResult(value.ScanStatusBlocked)

	_, err := r.Execute(context.Background(), result, "some content")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, shielderrors.ErrBlockedByPolicy) {
		t.Errorf("expected ErrBlockedByPolicy, got %v", err)
	}
}

// @sk-test 23-shield-reactions#T2.2: Test BlockReaction with nil result
func TestBlockReaction_NilResult(t *testing.T) {
	r := NewBlockReaction()
	_, err := r.Execute(context.Background(), nil, "text")
	if !errors.Is(err, shielderrors.ErrBlockedByPolicy) {
		t.Errorf("expected ErrBlockedByPolicy for nil result, got %v", err)
	}
}

// @sk-test remove-audit-incidents#T4.2: Test BlockReaction with clean status
func TestBlockReaction_CleanStatus(t *testing.T) {
	r := NewBlockReaction()
	result := entity.NewScanResult(value.ScanStatusClean)
	_, err := r.Execute(context.Background(), result, "text")
	if !errors.Is(err, shielderrors.ErrBlockedByPolicy) {
		t.Errorf("expected ErrBlockedByPolicy for clean result, got %v", err)
	}
}
