//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-test 30-shield-persistence#T4.1: TestProfileSaveAndFind integration (AC-001, AC-007)
func TestProfileSaveAndFind(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	txMgr := NewPGXTransactionManager(pool)
	dictRepo := NewPostgresDictionaryRepo(pool)
	profileRepo := NewPostgresProfileRepo(pool, dictRepo, txMgr)

	pid, _ := value.NewProfileID("p-integ-001")
	slug, _ := value.NewProfileSlug("integ-test-profile")
	tenant, _ := value.NewTenantID("t-integ-1")

	dictSlug, _ := value.NewProfileSlug("integ-test-profile")
	dictEntry := dictionary.NewDictionary(dictSlug, "blocklist", []interface{}{"secret", "admin"}, dictionary.MatchModeExact)

	pp := []preprocessor.PreprocessorDef{
		{Name: "csv-mask", Type: "csv", Rules: []preprocessor.Rule{{Columns: []string{"email"}, Mask: "full"}}},
	}

	profile := entity.NewProfile(pid, slug, tenant, "Integration Test Profile",
		entity.WithPreprocessors(pp),
		entity.WithDictionaries([]*dictionary.Dictionary{dictEntry}),
		entity.WithEnabled(true),
	)

	if err := profileRepo.Save(ctx, profile); err != nil {
		t.Fatalf("Save profile: %v", err)
	}

	loaded, err := profileRepo.FindBySlug(ctx, tenant, slug)
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil profile")
	}
	if loaded.Name() != "Integration Test Profile" {
		t.Errorf("unexpected name: %q", loaded.Name())
	}
	if !loaded.Enabled() {
		t.Error("expected enabled = true")
	}
	if len(loaded.Preprocessors()) != 1 {
		t.Errorf("expected 1 preprocessor, got %d", len(loaded.Preprocessors()))
	}
	if len(loaded.Dictionaries()) == 0 {
		t.Fatal("expected dictionaries to be loaded")
	}
	if len(loaded.Dictionaries()[0].Entries()) != 2 {
		t.Errorf("expected 2 dictionary entries, got %d", len(loaded.Dictionaries()[0].Entries()))
	}
}

// @sk-test 30-shield-persistence#T4.1: TestProfileDeleteCascade (AC-002, AC-007)
func TestProfileDeleteCascade(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	txMgr := NewPGXTransactionManager(pool)
	dictRepo := NewPostgresDictionaryRepo(pool)
	profileRepo := NewPostgresProfileRepo(pool, dictRepo, txMgr)
	incidentRepo := NewPostgresIncidentRepo(pool)

	pid, _ := value.NewProfileID("p-cascade-001")
	slug, _ := value.NewProfileSlug("cascade-delete-profile")
	tenant, _ := value.NewTenantID("t-cascade-1")

	profile := entity.NewProfile(pid, slug, tenant, "Cascade Test")
	if err := profileRepo.Save(ctx, profile); err != nil {
		t.Fatalf("Save profile: %v", err)
	}

	inc := entity.NewAuditIncident("", slug.String(), "req-1", "regex", nil, value.SeverityHigh, "block", nil, nil, tenant.String(), time.Now())
	if err := incidentRepo.Save(ctx, inc); err != nil {
		t.Fatalf("Save incident: %v", err)
	}

	if err := profileRepo.Delete(ctx, pid); err != nil {
		t.Fatalf("Delete profile: %v", err)
	}

	loaded, err := profileRepo.FindByID(ctx, pid)
	if err != nil {
		t.Fatalf("FindByID after delete: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil after delete")
	}

	incidents, err := incidentRepo.ListByProfile(ctx, pid)
	if err != nil {
		t.Fatalf("ListByProfile after delete: %v", err)
	}
	if len(incidents) != 0 {
		t.Errorf("expected 0 incidents after cascade delete, got %d", len(incidents))
	}
}

// @sk-test 30-shield-persistence#T4.1: TestIncidentSaveAndList (AC-004, AC-007)
func TestIncidentSaveAndList(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	txMgr := NewPGXTransactionManager(pool)
	dictRepo := NewPostgresDictionaryRepo(pool)
	profileRepo := NewPostgresProfileRepo(pool, dictRepo, txMgr)
	incidentRepo := NewPostgresIncidentRepo(pool)

	pid, _ := value.NewProfileID("p-inc-001")
	slug, _ := value.NewProfileSlug("incident-test-profile")
	tenant, _ := value.NewTenantID("t-inc-1")

	profile := entity.NewProfile(pid, slug, tenant, "Incident Test")
	if err := profileRepo.Save(ctx, profile); err != nil {
		t.Fatalf("Save profile: %v", err)
	}

	for i := 0; i < 3; i++ {
		inc := entity.NewAuditIncident("", slug.String(), "req-1", "regex", nil, value.SeverityHigh, "block", nil, nil, tenant.String(), time.Now())
		if err := incidentRepo.Save(ctx, inc); err != nil {
			t.Fatalf("Save incident %d: %v", i, err)
		}
	}

	incidents, err := incidentRepo.ListByProfile(ctx, pid)
	if err != nil {
		t.Fatalf("ListByProfile: %v", err)
	}
	if len(incidents) != 3 {
		t.Errorf("expected 3 incidents, got %d", len(incidents))
	}

	for _, inc := range incidents {
		if inc.ProfileSlug() != slug.String() {
			t.Errorf("unexpected profile slug: %q", inc.ProfileSlug())
		}
		if inc.Severity() != value.SeverityHigh {
			t.Errorf("unexpected severity: %v", inc.Severity())
		}
	}
}

// @sk-test 30-shield-persistence#T4.1: TestPoolHealthcheck (AC-005, AC-007)
func TestPoolHealthcheck(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool ping: %v", err)
	}
}

func setupPG(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("SHIELD_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("SHIELD_TEST_PG_DSN not set; skipping integration test")
	}

	cfg := &config.DatabaseConfig{
		DSN:             dsn,
		MaxConns:        5,
		MinConns:        1,
		MaxConnLifetime: 30 * time.Minute,
	}

	pool, err := NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}

	if err := RunMigrations(dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return pool
}
