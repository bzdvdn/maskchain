package reaction

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test remove-audit-incidents#T4.1: Test AlertReaction logs result info without persisting (AC-014)
func TestAlertReaction_LogsResult(t *testing.T) {
	log := zap.NewNop()
	ar := NewAlertReaction(log)
	result := entity.NewScanResult(value.ScanStatusSuspicious)

	out, err := ar.Execute(context.Background(), result, "original content")
	if err != nil {
		t.Fatal(err)
	}
	if out != "original content" {
		t.Errorf("expected unchanged text, got %q", out)
	}
}

// @sk-test remove-audit-incidents#T4.1: Test AlertReaction with nil result
func TestAlertReaction_NilResult(t *testing.T) {
	log := zap.NewNop()
	ar := NewAlertReaction(log)

	out, err := ar.Execute(context.Background(), nil, "text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "text" {
		t.Errorf("expected original text, got %q", out)
	}
}
