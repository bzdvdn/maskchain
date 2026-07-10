package reaction

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type mockIncidentRepo struct {
	mu      sync.Mutex
	saved   []entity.Incident
	saveErr error
}

func (r *mockIncidentRepo) Save(_ context.Context, incident *entity.Incident) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = append(r.saved, *incident)
	return nil
}

func (r *mockIncidentRepo) FindByID(_ context.Context, _ string) (*entity.Incident, error) {
	return nil, errors.New("not implemented")
}

func (r *mockIncidentRepo) ListByProfile(_ context.Context, _ value.ProfileID) ([]*entity.Incident, error) {
	return nil, errors.New("not implemented")
}

func (r *mockIncidentRepo) ListByTenant(_ context.Context, _ value.TenantID) ([]*entity.Incident, error) {
	return nil, errors.New("not implemented")
}

// @sk-test 23-shield-reactions#T2.4: Test AlertReaction logs incidents without modifying text (AC-004)
func TestAlertReaction_LogsIncidents(t *testing.T) {
	repo := &mockIncidentRepo{}
	ar := NewAlertReaction(repo)

	incidents := []entity.Incident{
		*entity.NewIncident("det1", mustPatternID("p1"), value.SeverityHigh, "sensitive", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	out, err := ar.Execute(context.Background(), result, "original content")
	if err != nil {
		t.Fatal(err)
	}
	if out != "original content" {
		t.Errorf("expected unchanged text, got %q", out)
	}
	if len(repo.saved) != 1 {
		t.Errorf("expected 1 saved incident, got %d", len(repo.saved))
	}
}

// @sk-test 23-shield-reactions#T2.4: Test AlertReaction with nil result
func TestAlertReaction_NilResult(t *testing.T) {
	repo := &mockIncidentRepo{}
	ar := NewAlertReaction(repo)

	out, err := ar.Execute(context.Background(), nil, "text")
	if err != nil {
		t.Fatal(err)
	}
	if out != "text" {
		t.Errorf("expected original text, got %q", out)
	}
	if len(repo.saved) != 0 {
		t.Errorf("expected no saved incidents, got %d", len(repo.saved))
	}
}

// @sk-test 23-shield-reactions#T2.4: Test AlertReaction returns error on repo failure
func TestAlertReaction_RepoError(t *testing.T) {
	repo := &mockIncidentRepo{saveErr: errors.New("db down")}
	ar := NewAlertReaction(repo)

	incidents := []entity.Incident{
		*entity.NewIncident("det1", mustPatternID("p1"), value.SeverityHigh, "sensitive", 0),
	}
	result := entity.NewScanResult(value.ScanStatusSuspicious, incidents)

	_, err := ar.Execute(context.Background(), result, "text")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
