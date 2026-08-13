package config

import (
	"testing"
)

func TestDefaultConfig_ComplianceDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Compliance == nil {
		t.Fatal("expected compliance config to be set by defaults")
	}
	if cfg.Compliance.PresetDir != defaultCompliancePresetDir {
		t.Errorf("expected preset_dir=%q, got %q", defaultCompliancePresetDir, cfg.Compliance.PresetDir)
	}
	if len(cfg.Compliance.EnabledPacks) != 0 {
		t.Errorf("expected no enabled packs by default, got %v", cfg.Compliance.EnabledPacks)
	}
}

func TestValidateCompliance_RequiresPresetDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Compliance.PresetDir = ""
	if err := validateCompliance(cfg); err == nil {
		t.Fatal("expected error when compliance.preset_dir is empty")
	}
}

func TestValidateCompliance_RejectsEmptyPackKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Compliance.EnabledPacks = []string{""}
	if err := validateCompliance(cfg); err == nil {
		t.Fatal("expected error when an enabled pack key is empty")
	}
}

func TestValidateCompliance_Valid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Compliance.EnabledPacks = []string{"HIPAA", "GDPR"}
	if err := validateCompliance(cfg); err != nil {
		t.Fatalf("expected valid compliance config to pass: %v", err)
	}
}
