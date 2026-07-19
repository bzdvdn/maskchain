package reaction

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test remove-audit-incidents#T4.2: Test ReactionPipeline routes ReactionBlock to BlockReaction (AC-005)
func TestReactionPipeline_RoutesBlock(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	p := NewDefaultReactionPipeline(NewBlockReaction(), NewRedactReaction(log), NewAlertReaction(log))

	result := entity.NewScanResult(value.ScanStatusBlocked)

	_, err := p.Execute(context.Background(), entity.ReactionBlock, result, "content")
	if err == nil {
		t.Fatal("expected error for ReactionBlock")
	}
	if !errors.Is(err, shielderrors.ErrBlockedByPolicy) {
		t.Errorf("expected ErrBlockedByPolicy, got %v", err)
	}
}

// @sk-test remove-audit-incidents#T4.2: Test ReactionPipeline routes ReactionLog to RedactReaction (AC-005)
func TestReactionPipeline_RoutesLog(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	p := NewDefaultReactionPipeline(NewBlockReaction(), NewRedactReaction(log), NewAlertReaction(log))

	result := entity.NewScanResult(value.ScanStatusSuspicious)

	out, err := p.Execute(context.Background(), entity.ReactionLog, result, "email: user@site.com")
	if err != nil {
		t.Fatal(err)
	}
	if out != "email: user@site.com" {
		t.Errorf("expected unchanged text, got %q", out)
	}
}

// @sk-test remove-audit-incidents#T4.2: Test ReactionPipeline routes ReactionReview to AlertReaction (AC-005)
func TestReactionPipeline_RoutesReview(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	p := NewDefaultReactionPipeline(NewBlockReaction(), NewRedactReaction(log), NewAlertReaction(log))

	result := entity.NewScanResult(value.ScanStatusSuspicious)

	out, err := p.Execute(context.Background(), entity.ReactionReview, result, "original content")
	if err != nil {
		t.Fatal(err)
	}
	if out != "original content" {
		t.Errorf("expected original text, got %q", out)
	}
}

// @sk-test 23-shield-reactions#T2.5: Test ReactionPipeline returns text unchanged for ReactionAllow (AC-005)
func TestReactionPipeline_RoutesAllow(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	p := NewDefaultReactionPipeline(NewBlockReaction(), NewRedactReaction(log), NewAlertReaction(log))

	out, err := p.Execute(context.Background(), entity.ReactionAllow, nil, "content")
	if err != nil {
		t.Fatal(err)
	}
	if out != "content" {
		t.Errorf("expected original text, got %q", out)
	}
}
