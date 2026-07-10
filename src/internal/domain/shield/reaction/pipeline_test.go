package reaction

import (
	"context"
	"errors"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 23-shield-reactions#T2.5: Test ReactionPipeline routes ReactionBlock to BlockReaction (AC-005)
func TestReactionPipeline_RoutesBlock(t *testing.T) {
	p := NewDefaultReactionPipeline(NewBlockReaction(), NewRedactReaction(), NewAlertReaction(&mockIncidentRepo{}))

	incidents := []entity.Incident{
		*entity.NewIncident("email", mustPatternID("p1"), value.SeverityCritical, "secret", 0),
	}
	result := entity.NewScanResult(value.ScanStatusBlocked, incidents)

	_, err := p.Execute(context.Background(), entity.ReactionBlock, result, "content")
	if err == nil {
		t.Fatal("expected error for ReactionBlock")
	}
	if !errors.Is(err, shielderrors.ErrBlockedByPolicy) {
		t.Errorf("expected ErrBlockedByPolicy, got %v", err)
	}
}

// @sk-test 23-shield-reactions#T2.5: Test ReactionPipeline routes ReactionLog to RedactReaction (AC-005)
func TestReactionPipeline_RoutesLog(t *testing.T) {
	p := NewDefaultReactionPipeline(NewBlockReaction(), NewRedactReaction(), NewAlertReaction(&mockIncidentRepo{}))

	incidents := []entity.Incident{
		*entity.NewIncident("email", mustPatternID("p1"), value.SeverityLow, "user@site.com", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := p.Execute(context.Background(), entity.ReactionLog, result, "email: user@site.com")
	if err != nil {
		t.Fatal(err)
	}
	expected := "email: *************"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

// @sk-test 23-shield-reactions#T2.5: Test ReactionPipeline routes ReactionReview to AlertReaction (AC-005)
func TestReactionPipeline_RoutesReview(t *testing.T) {
	repo := &mockIncidentRepo{}
	p := NewDefaultReactionPipeline(NewBlockReaction(), NewRedactReaction(), NewAlertReaction(repo))

	incidents := []entity.Incident{
		*entity.NewIncident("det", mustPatternID("p1"), value.SeverityHigh, "sensitive", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := p.Execute(context.Background(), entity.ReactionReview, result, "original content")
	if err != nil {
		t.Fatal(err)
	}
	if out != "original content" {
		t.Errorf("expected original text, got %q", out)
	}
	if len(repo.saved) != 1 {
		t.Errorf("expected 1 saved incident from AlertReaction, got %d", len(repo.saved))
	}
}

// @sk-test 23-shield-reactions#T2.5: Test ReactionPipeline returns text unchanged for ReactionAllow (AC-005)
func TestReactionPipeline_RoutesAllow(t *testing.T) {
	p := NewDefaultReactionPipeline(NewBlockReaction(), NewRedactReaction(), NewAlertReaction(&mockIncidentRepo{}))

	out, err := p.Execute(context.Background(), entity.ReactionAllow, nil, "content")
	if err != nil {
		t.Fatal(err)
	}
	if out != "content" {
		t.Errorf("expected original text, got %q", out)
	}
}
