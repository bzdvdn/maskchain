package reaction

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test remove-audit-incidents#T4.2: Test RedactReaction returns text unchanged (AC-002)
func TestRedactReaction_ReturnsTextUnchanged(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	r := NewRedactReaction(log)
	result := entity.NewScanResult(value.ScanStatusSuspicious)

	out, err := r.Execute(context.Background(), result, "email: user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if out != "email: user@example.com" {
		t.Errorf("expected unchanged text, got %q", out)
	}
}

// @sk-test remove-audit-incidents#T4.2: Test RedactReaction with nil result
func TestRedactReaction_NilResult(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	r := NewRedactReaction(log)
	out, err := r.Execute(context.Background(), nil, "some text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "some text" {
		t.Errorf("expected original text, got %q", out)
	}
}

// @sk-test remove-audit-incidents#T4.2: Test RedactReaction with clean status
func TestRedactReaction_CleanStatus(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	r := NewRedactReaction(log)
	result := entity.NewScanResult(value.ScanStatusClean)

	out, err := r.Execute(context.Background(), result, "some text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "some text" {
		t.Errorf("expected original text, got %q", out)
	}
}
