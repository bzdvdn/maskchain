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

// @sk-test 110-provider-adapters#T1.2: TestProviderConfig_APITypeOpenAI (AC-007)
func TestProviderConfig_APITypeOpenAI(t *testing.T) {
	cfg := testProviderConfig(t, "openai", "")
	if cfg.Routing.Providers[0].APIType != "openai" {
		t.Errorf("expected APIType=openai, got %q", cfg.Routing.Providers[0].APIType)
	}
}

// @sk-test 110-provider-adapters#T1.2: TestProviderConfig_APITypeAnthropic (AC-007)
func TestProviderConfig_APITypeAnthropic(t *testing.T) {
	cfg := testProviderConfig(t, "anthropic", "sk-ant-xxx")
	if cfg.Routing.Providers[0].APIType != "anthropic" {
		t.Errorf("expected APIType=anthropic, got %q", cfg.Routing.Providers[0].APIType)
	}
}

// @sk-test 110-provider-adapters#T1.2: TestProviderConfig_APIKey (AC-008)
func TestProviderConfig_APIKey(t *testing.T) {
	cfg := testProviderConfig(t, "openai", "sk-test-key-123")
	if cfg.Routing.Providers[0].APIKey != "sk-test-key-123" {
		t.Errorf("expected APIKey=sk-test-key-123, got %q", cfg.Routing.Providers[0].APIKey)
	}
}

// testProviderConfig creates a temp config with given api_type and api_key, returns parsed Config.
func testProviderConfig(t *testing.T, apiType, apiKey string) *Config {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: " + apiType + "\n      api_key: " + apiKey + "\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseAndLoadConfig([]string{"--config=" + cfgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return cfg
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
