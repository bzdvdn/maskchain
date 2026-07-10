package reaction

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 23-shield-reactions#T2.3: Test RedactReaction replaces fragment with asterisks (AC-002)
func TestRedactReaction_ReplacesFragment(t *testing.T) {
	r := NewRedactReaction()
	incidents := []entity.Incident{
		*entity.NewIncident("email", mustPatternID("p1"), value.SeverityMedium, "user@example.com", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := r.Execute(context.Background(), result, "email: user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	expected := "email: ****************"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

// @sk-test 23-shield-reactions#T2.3: Test RedactReaction with nil result
func TestRedactReaction_NilResult(t *testing.T) {
	r := NewRedactReaction()
	out, err := r.Execute(context.Background(), nil, "some text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "some text" {
		t.Errorf("expected original text, got %q", out)
	}
}

// @sk-test 23-shield-reactions#T2.3: Test RedactReaction with empty incidents
func TestRedactReaction_EmptyIncidents(t *testing.T) {
	r := NewRedactReaction()
	result := entity.NewScanResult(value.ScanStatusClean, nil)

	out, err := r.Execute(context.Background(), result, "some text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "some text" {
		t.Errorf("expected original text, got %q", out)
	}
}

// @sk-test 23-shield-reactions#T2.3: Test RedactReaction multiple fragments
func TestRedactReaction_MultipleFragments(t *testing.T) {
	r := NewRedactReaction()
	incidents := []entity.Incident{
		*entity.NewIncident("email", mustPatternID("p1"), value.SeverityMedium, "alice@example.com", 0),
		*entity.NewIncident("email", mustPatternID("p2"), value.SeverityLow, "+1-555-1234", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := r.Execute(context.Background(), result, "Contact: alice@example.com, Phone: +1-555-1234")
	if err != nil {
		t.Fatal(err)
	}
	expected := "Contact: *****************, Phone: ***********"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

// @sk-test 23-shield-reactions#T2.3: Test RedactReaction empty fragment is skipped
func TestRedactReaction_EmptyFragment(t *testing.T) {
	r := NewRedactReaction()
	incidents := []entity.Incident{
		*entity.NewIncident("det", mustPatternID("p1"), value.SeverityLow, "", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := r.Execute(context.Background(), result, "some text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "some text" {
		t.Errorf("expected original text, got %q", out)
	}
}
