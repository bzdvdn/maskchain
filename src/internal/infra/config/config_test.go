package config

import (
	"os"
	"path/filepath"
	"testing"
)

// @sk-test 01-config-bootstrap#T3.1: TestLoadConfig_EnvOverride (AC-001)
func TestLoadConfig_EnvOverride(t *testing.T) {
	t.Setenv("CONFIG_LOG_LEVEL", "debug")

	cfg, err := ParseAndLoadConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log.level=debug, got %q", cfg.Log.Level)
	}
}

// @sk-test 01-config-bootstrap#T3.1: TestLoadConfig_CLIOverride (AC-002)
func TestLoadConfig_CLIOverride(t *testing.T) {
	t.Setenv("CONFIG_LOG_LEVEL", "warn")

	cfg, err := ParseAndLoadConfig([]string{"--log-level=error"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "error" {
		t.Errorf("expected log.level=error, got %q", cfg.Log.Level)
	}
}

// @sk-test 01-config-bootstrap#T3.1: TestLoadConfig_RequiredValidation (AC-003)
func TestLoadConfig_RequiredValidation(t *testing.T) {
	_, err := ParseAndLoadConfig([]string{})
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
	if err.Error() != "missing required field: log.level" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// @sk-test 01-config-bootstrap#T3.1: TestLoadConfig_CustomConfigPath (AC-005)
func TestLoadConfig_CustomConfigPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "custom.yaml")
	content := []byte("log:\n  level: info\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseAndLoadConfig([]string{"--config=" + cfgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("expected log.level=info, got %q", cfg.Log.Level)
	}
}

// @sk-test 30-shield-persistence#T1.3: TestDatabaseConfig_PoolDefaults (AC-005)
func TestDatabaseConfig_PoolDefaults(t *testing.T) {
	t.Setenv("CONFIG_LOG_LEVEL", "debug")
	t.Setenv("CONFIG_DATABASE_DSN", "postgres://localhost:5432/test")

	cfg, err := ParseAndLoadConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DB == nil {
		t.Fatal("expected DB config, got nil")
	}
	if cfg.DB.MaxConns != 25 {
		t.Errorf("expected MaxConns=25, got %d", cfg.DB.MaxConns)
	}
	if cfg.DB.MinConns != 1 {
		t.Errorf("expected MinConns=1, got %d", cfg.DB.MinConns)
	}
	if cfg.DB.MaxConnLifetime != 30*60_000_000_000 {
		t.Errorf("expected MaxConnLifetime=30m, got %v", cfg.DB.MaxConnLifetime)
	}
}
