package compliance

import (
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

const testPresetDir = "../../../../specs/active/401-compliance-packs/testdata"

func TestLoadPacksFromDir_RegistersByKey(t *testing.T) {
	catalog := NewCatalog([]entity.DetectorType{
		entity.DetectorTypeRegex,
		entity.DetectorTypeDictionary,
		entity.DetectorTypePromptInjection,
	}, []entity.Reaction{entity.ReactionAllow, entity.ReactionBlock, entity.ReactionReview, entity.ReactionLog})

	reg, errs := LoadPacksFromDir(testPresetDir, catalog)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (broken.yaml), got %d: %v", len(errs), errs)
	}

	packs := reg.Packs()
	for _, want := range []string{"HIPAA", "PCI DSS", "GDPR", "Legal"} {
		if _, ok := packs[want]; !ok {
			t.Errorf("expected pack %q to be registered", want)
		}
	}
	if _, ok := packs["Broken"]; ok {
		t.Errorf("broken pack must not be registered")
	}
}

func TestLoadPacksFromDir_ValidatesDetectorTypes(t *testing.T) {
	catalog := NewCatalog([]entity.DetectorType{entity.DetectorTypeRegex}, []entity.Reaction{entity.ReactionBlock})
	reg, errs := LoadPacksFromDir(testPresetDir, catalog)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for presets referencing unknown detectors")
	}
	if reg.Pack("HIPAA") != nil {
		t.Fatal("HIPAA references dictionary/prompt_injection which are not in catalog")
	}
}

func TestPack_ValidateRejectsUnknownElements(t *testing.T) {
	catalog := NewCatalog([]entity.DetectorType{entity.DetectorTypeRegex}, []entity.Reaction{entity.ReactionBlock})
	pack := &Pack{
		Key: "Broken",
		Rules: []PackRule{
			{DetectorType: entity.DetectorType("nope"), Reaction: entity.ReactionBlock},
		},
	}
	if err := pack.Validate(catalog); err == nil {
		t.Fatal("expected validation error for unknown detector type")
	}
}

func TestPack_ValidateEmptyKey(t *testing.T) {
	catalog := NewCatalog(nil, nil)
	pack := &Pack{}
	if err := pack.Validate(catalog); err == nil {
		t.Fatal("expected validation error for empty pack key")
	}
}

func TestPack_RuleForAndDetectorTypes(t *testing.T) {
	pack := &Pack{
		Key: "HIPAA",
		Rules: []PackRule{
			{DetectorType: entity.DetectorTypeDictionary, Reaction: entity.ReactionBlock, Masking: true},
			{DetectorType: entity.DetectorTypeRegex, Reaction: entity.ReactionBlock, Masking: true},
		},
	}
	if r := pack.RuleFor(entity.DetectorTypeRegex); r == nil || !r.Masking {
		t.Fatalf("expected masking rule for regex, got %+v", r)
	}
	if r := pack.RuleFor(entity.DetectorType("none")); r != nil {
		t.Fatalf("expected nil rule for unknown detector")
	}
	types := pack.DetectorTypes()
	if len(types) != 2 || types[0] != entity.DetectorTypeDictionary || types[1] != entity.DetectorTypeRegex {
		t.Fatalf("expected sorted detector types [dictionary regex], got %v", types)
	}
}

func TestNewCatalogFromRegistry(t *testing.T) {
	reg := detector.NewDetectorRegistry()
	_ = reg.Register(entity.DetectorTypeRegex, detector.NewCompositeDetector())
	cat := NewCatalogFromRegistry(reg)
	p := &Pack{
		Key:   "X",
		Rules: []PackRule{{DetectorType: entity.DetectorTypeRegex, Reaction: entity.ReactionBlock}},
	}
	if err := p.Validate(cat); err != nil {
		t.Fatalf("expected regex to be valid via registry catalog: %v", err)
	}
}

// TestLoadPacksFromDir_NewYAMLBecomesPack proves extensibility: adding a new
// preset YAML file exposes a new pack without any code change (AC-004).
func TestLoadPacksFromDir_NewYAMLBecomesPack(t *testing.T) {
	catalog := NewCatalog([]entity.DetectorType{
		entity.DetectorTypeRegex,
		entity.DetectorTypeDictionary,
		entity.DetectorTypePromptInjection,
	}, []entity.Reaction{entity.ReactionAllow, entity.ReactionBlock, entity.ReactionReview, entity.ReactionLog})

	reg, errs := LoadPacksFromDir(testPresetDir, catalog)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (broken.yaml), got %d: %v", len(errs), errs)
	}

	pack := reg.Pack("SOC 2")
	if pack == nil {
		t.Fatal("expected SOC 2 pack to be registered from soc2.yaml without code change")
	}
	if pack.Name != "Service Organization Control 2" {
		t.Fatalf("unexpected pack name %q", pack.Name)
	}
	if len(pack.Rules) != 2 {
		t.Fatalf("expected 2 rules in SOC 2, got %d", len(pack.Rules))
	}
	if pack.RuleFor(entity.DetectorTypePromptInjection) == nil {
		t.Fatal("expected prompt_injection rule in SOC 2")
	}
}
