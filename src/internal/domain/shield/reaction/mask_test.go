package reaction

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test remove-audit-incidents#T4.2: Test MaskReaction returns text unchanged (AC-003)
func TestMaskReaction_ReturnsTextUnchanged(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	mr := NewMaskReaction(nil, log)

	result := entity.NewScanResult(value.ScanStatusSuspicious)

	out, err := mr.Execute(context.Background(), result, "email: user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if out != "email: user@example.com" {
		t.Errorf("expected unchanged text, got %q", out)
	}
}

// @sk-test remove-audit-incidents#T4.2: Test MaskReaction with nil result
func TestMaskReaction_NilResult(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	mr := NewMaskReaction(nil, log)

	out, err := mr.Execute(context.Background(), nil, "text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "text" {
		t.Errorf("expected original text, got %q", out)
	}
}

// @sk-test remove-audit-incidents#T4.2: Test MaskReaction with clean status
func TestMaskReaction_CleanStatus(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	mr := NewMaskReaction(nil, log)

	result := entity.NewScanResult(value.ScanStatusClean)

	out, err := mr.Execute(context.Background(), result, "text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "text" {
		t.Errorf("expected original text, got %q", out)
	}
}
