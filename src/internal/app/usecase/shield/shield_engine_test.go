package shield

import (
	"context"
	"errors"
	"fmt"
	"testing"

	shielddomain "github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/reaction"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/service"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 50-shield-engine#T2.2: TestFullPipeline (AC-001)
func TestShieldEngine_FullPipeline(t *testing.T) {
	ctx := context.Background()

	registry, _ := setupRegistry(t)
	repos := setupRepos(t, registry)

	engine := repos.engine
	text := "email,phone,notes\ntest@example.com,+1-555-123-4567,handle secret123 ended\n"

	resp, err := engine.Scan(ctx, ScanRequest{Text: text, ProfileSlug: "test-profile"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status() != value.ScanStatusSuspicious {
		t.Errorf("expected status suspicious, got %v", resp.Status())
	}

	if len(resp.Incidents()) < 2 {
		t.Errorf("expected at least 2 incidents (dictionary + PII), got %d", len(resp.Incidents()))
	}

	for _, orig := range []string{"test@example.com", "+1-555-123-4567", "secret123"} {
		if contains(resp.ProcessedText, orig) {
			t.Errorf("processed text should not contain %q: %q", orig, resp.ProcessedText)
		}
	}
}

// @sk-test 50-shield-engine#T2.2: TestProfileNotFound (AC-004)
func TestShieldEngine_ProfileNotFound(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)
	repos := setupRepos(t, registry)

	_, err := repos.engine.Scan(ctx, ScanRequest{Text: "hello", ProfileSlug: "does-not-exist"})
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound, got %v", err)
	}
}

// @sk-test 50-shield-engine#T2.2: TestProfileDisabled (AC-004 variant)
func TestShieldEngine_ProfileDisabled(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)

	repos := setupRepos(t, registry)
	slug, _ := value.NewProfileSlug("disabled-prof")

	profID, _ := value.NewProfileID("disabled")
	disabledProfile := entity.NewProfile(
		profID, slug, repos.tenantID, "disabled profile",
		entity.WithEnabled(false),
	)
	repos.repo.Save(ctx, disabledProfile)

	_, err := repos.engine.Scan(ctx, ScanRequest{Text: "hello", ProfileSlug: "disabled-prof"})
	if !errors.Is(err, ErrProfileDisabled) {
		t.Errorf("expected ErrProfileDisabled, got %v", err)
	}
}

// @sk-test 50-shield-engine#T2.2: TestEmptyPipeline (AC-005)
func TestShieldEngine_EmptyPipeline(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)

	repos := setupRepos(t, registry)
	slug, _ := value.NewProfileSlug("empty-prof")

	profID, _ := value.NewProfileID("empty")
	profile := entity.NewProfile(profID, slug, repos.tenantID, "empty pipeline")
	repos.repo.Save(ctx, profile)

	resp, err := repos.engine.Scan(ctx, ScanRequest{Text: "some text", ProfileSlug: "empty-prof"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status() != value.ScanStatusClean {
		t.Errorf("expected clean status for empty pipeline, got %v", resp.Status())
	}
	if len(resp.Incidents()) != 0 {
		t.Errorf("expected 0 incidents for empty pipeline, got %d", len(resp.Incidents()))
	}
}

// @sk-test 50-shield-engine#T3.2: TestPlaceholderMasking (AC-002)
func TestShieldEngine_PlaceholderMasking(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)
	repos := setupMaskRepos(t, registry)
	engine := repos.engine

	text := "email,phone\nuser@example.com,+1-555-123-4567\n"
	resp, err := engine.Scan(ctx, ScanRequest{Text: text, ProfileSlug: "mask-profile"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Replacements) == 0 {
		t.Fatal("expected non-empty Replacements")
	}

	foundCSV := false
	for k, v := range resp.Replacements {
		if len(k) < 5 || k[:5] != "{{csv" {
			continue
		}
		foundCSV = true
		if v != "user@example.com" {
			t.Errorf("expected original value %q, got %q", "user@example.com", v)
		}
	}
	if !foundCSV {
		t.Error("expected a csv-style placeholder in Replacements")
	}

	restored := resp.ProcessedText
	for k, v := range resp.Replacements {
		restored = replaceAll(restored, k, v)
	}
	if restored != text {
		t.Errorf("round-trip restore failed:\n  original: %q\n  restored: %q", text, restored)
	}
}

// @sk-test 50-shield-engine#T3.2: TestAllPlaceholderFormats (AC-006)
func TestShieldEngine_AllPlaceholderFormats(t *testing.T) {
	ctx := context.Background()
	registry, _ := setupRegistry(t)
	repos := setupMaskRepos(t, registry)
	engine := repos.engine

	text := "email,phone,notes\nuser@example.com,+1-555-123-4567,handle secret123 extra\n"
	resp, err := engine.Scan(ctx, ScanRequest{Text: text, ProfileSlug: "mask-profile"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasCSV := stringsContains(resp.ProcessedText, "{{csv.")
	hasP := stringsContains(resp.ProcessedText, "{{p.")
	hasDict := stringsContains(resp.ProcessedText, "{{dict.")

	if !hasCSV {
		t.Error("expected {{csv.*}} placeholder in processed text")
	}
	if !hasP {
		t.Error("expected {{p.*}} placeholder in processed text")
	}
	if !hasDict {
		t.Error("expected {{dict.*}} placeholder in processed text")
	}
}

// --- test helpers ---

type testRepos struct {
	repo     *memProfileRepo
	engine   *ShieldEngine
	tenantID value.TenantID
}

func setupRegistry(t *testing.T) (*detector.DetectorRegistry, entity.DetectorType) {
	t.Helper()
	reg := detector.NewDetectorRegistry()
	pii, err := detector.NewPIIDetector()
	if err != nil {
		t.Fatalf("new PII detector: %v", err)
	}
	piiType := entity.DetectorType("pii")
	if err := reg.Register(piiType, pii); err != nil {
		t.Fatalf("register PII: %v", err)
	}
	return reg, piiType
}

func setupRepos(t *testing.T, registry *detector.DetectorRegistry) *testRepos {
	t.Helper()
	tenantID, _ := value.NewTenantID("test-tenant")
	factory := NewScanPipelineFactory(registry)
	evaluator := service.NewPolicyEvaluator()
	incidentRepo := &stubIncidentRepo{}
	reactionPipeline := reaction.NewDefaultReactionPipeline(
		reaction.NewBlockReaction(),
		reaction.NewRedactReaction(),
		reaction.NewAlertReaction(incidentRepo),
	)
	repo := &memProfileRepo{
		profiles: make(map[string]*entity.Profile),
	}
	useCase := NewScanUseCase(repo, factory, evaluator, reactionPipeline, tenantID)
	engine := NewShieldEngine(useCase)

	slug, _ := value.NewProfileSlug("test-profile")
	profID, _ := value.NewProfileID("test-profile-id")

	patID, _ := value.NewPatternID("pii-catch-all")
	pattern, _ := entity.NewPattern(patID, ".*", "catch all")
	piiDetector, _ := entity.NewDetector(
		"pii-detector",
		entity.DetectorType("pii"),
		[]entity.Pattern{*pattern},
		value.SeverityMedium,
	)

	dict := dictionary.NewDictionary(
		slug,
		"test-dict",
		[]interface{}{"secret123"},
		dictionary.MatchModeExact,
	)

	csvPreprocessor := preprocessor.PreprocessorDef{
		Name: "csv-mask-email",
		Type: "csv",
		Rules: []preprocessor.Rule{
			{Columns: []string{"email"}, Mask: preprocessor.MaskModeFull},
		},
	}

	profile := entity.NewProfile(
		profID, slug, tenantID, "test profile",
		entity.WithDetectors([]entity.Detector{*piiDetector}),
		entity.WithDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithPreprocessors([]preprocessor.PreprocessorDef{csvPreprocessor}),
		entity.WithEnabled(true),
	)

	repo.profiles[slug.String()] = profile

	return &testRepos{
		repo:     repo,
		engine:   engine,
		tenantID: tenantID,
	}
}

// in-memory profile repository

type memProfileRepo struct {
	profiles map[string]*entity.Profile
}

func (r *memProfileRepo) Save(_ context.Context, profile *entity.Profile) error {
	r.profiles[profile.Slug().String()] = profile
	return nil
}

func (r *memProfileRepo) FindByID(_ context.Context, id value.ProfileID) (*entity.Profile, error) {
	p, ok := r.profiles[id.String()]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return p, nil
}

func (r *memProfileRepo) FindBySlug(_ context.Context, _ value.TenantID, slug value.ProfileSlug) (*entity.Profile, error) {
	p, ok := r.profiles[slug.String()]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return p, nil
}

func (r *memProfileRepo) ListByTenant(_ context.Context, _ value.TenantID) ([]*entity.Profile, error) {
	var list []*entity.Profile
	for _, p := range r.profiles {
		list = append(list, p)
	}
	return list, nil
}

func (r *memProfileRepo) Delete(_ context.Context, id value.ProfileID) error {
	for k, p := range r.profiles {
		if p.ID() == id {
			delete(r.profiles, k)
			return nil
		}
	}
	return fmt.Errorf("not found")
}

// stub incident repository

type stubIncidentRepo struct {
	incidents []*entity.Incident
}

func (r *stubIncidentRepo) Save(_ context.Context, incident *entity.Incident) error {
	r.incidents = append(r.incidents, incident)
	return nil
}

func (r *stubIncidentRepo) FindByID(_ context.Context, _ string) (*entity.Incident, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *stubIncidentRepo) ListByProfile(_ context.Context, _ value.ProfileID) ([]*entity.Incident, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *stubIncidentRepo) ListByTenant(_ context.Context, _ value.TenantID) ([]*entity.Incident, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *stubIncidentRepo) List(_ context.Context, _ shielddomain.IncidentFilter) ([]*entity.Incident, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

// small helpers
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func replaceAll(s, old, new string) string {
	for stringsContains(s, old) {
		s = stringsReplace(s, old, new, 1)
	}
	return s
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringsReplace(s, old, new string, n int) string {
	if n == 0 || old == "" {
		return s
	}
	idx := -1
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			idx = i
			break
		}
	}
	if idx < 0 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

// setup for mask-mode tests
func setupMaskRepos(t *testing.T, registry *detector.DetectorRegistry) *testRepos {
	t.Helper()
	tenantID, _ := value.NewTenantID("mask-tenant")
	factory := NewScanPipelineFactory(registry)
	evaluator := service.NewPolicyEvaluator()
	incidentRepo := &stubIncidentRepo{}
	reactionPipeline := reaction.NewDefaultReactionPipeline(
		reaction.NewBlockReaction(),
		reaction.NewRedactReaction(),
		reaction.NewAlertReaction(incidentRepo),
	)
	repo := &memProfileRepo{
		profiles: make(map[string]*entity.Profile),
	}
	useCase := NewScanUseCase(repo, factory, evaluator, reactionPipeline, tenantID, WithMaskMode())
	engine := NewShieldEngine(useCase)

	slug, _ := value.NewProfileSlug("mask-profile")
	profID, _ := value.NewProfileID("mask-profile-id")

	patID, _ := value.NewPatternID("pii-mask-pat")
	pattern, _ := entity.NewPattern(patID, ".*", "catch all")
	piiDetector, _ := entity.NewDetector(
		"pii-detector",
		entity.DetectorType("pii"),
		[]entity.Pattern{*pattern},
		value.SeverityMedium,
	)

	dict := dictionary.NewDictionary(
		slug,
		"test-dict",
		[]interface{}{"secret123"},
		dictionary.MatchModeExact,
	)

	csvPreprocessor := preprocessor.PreprocessorDef{
		Name: "csv-mask-email",
		Type: "csv",
		Rules: []preprocessor.Rule{
			{Columns: []string{"email"}, Mask: preprocessor.MaskModeFull},
		},
	}

	profile := entity.NewProfile(
		profID, slug, tenantID, "mask profile",
		entity.WithDetectors([]entity.Detector{*piiDetector}),
		entity.WithDictionaries([]*dictionary.Dictionary{dict}),
		entity.WithPreprocessors([]preprocessor.PreprocessorDef{csvPreprocessor}),
		entity.WithEnabled(true),
	)

	repo.profiles[slug.String()] = profile

	return &testRepos{
		repo:     repo,
		engine:   engine,
		tenantID: tenantID,
	}
}
