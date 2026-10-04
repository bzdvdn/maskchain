package config

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if len(cfg.Routing.Providers[0].APIKeys) == 0 || cfg.Routing.Providers[0].APIKeys[0] != "sk-test-key-123" {
		t.Errorf("expected APIKeys[0]=sk-test-key-123, got %v", cfg.Routing.Providers[0].APIKeys)
	}
}

// testProviderConfig creates a temp config with given api_type and api_key, returns parsed Config.
func testProviderConfig(t *testing.T, apiType, apiKey string) *Config {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: " + apiType + "\n      api_keys:\n        - " + apiKey + "\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseAndLoadConfig([]string{"--config=" + cfgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return cfg
}

// @sk-test 111-provider-auth-and-config#T4.1: TestProviderConfig_APIKeys — чтение api_keys из YAML (AC-001)
func TestProviderConfig_APIKeys(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n      api_keys:\n        - sk-abc123\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseAndLoadConfig([]string{"--config=" + cfgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Routing.Providers[0].APIKeys) != 1 || cfg.Routing.Providers[0].APIKeys[0] != "sk-abc123" {
		t.Errorf("expected APIKeys=[sk-abc123], got %v", cfg.Routing.Providers[0].APIKeys)
	}
}

// @sk-test 111-provider-auth-and-config#T4.1: TestProviderConfig_EnvAPIKeys — чтение api_key из env + fallback (AC-002)
func TestProviderConfig_EnvAPIKeys(t *testing.T) {
	t.Setenv("CONFIG_ROUTING_PROVIDERS_0_API_KEY", "sk-env-key")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// No api_keys in YAML, only name+base_url+api_type — fallback должен скопировать api_key → api_keys[0]
	content := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseAndLoadConfig([]string{"--config=" + cfgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Routing.Providers[0].APIKeys) != 1 || cfg.Routing.Providers[0].APIKeys[0] != "sk-env-key" {
		t.Errorf("expected APIKeys=[sk-env-key] via fallback, got %v", cfg.Routing.Providers[0].APIKeys)
	}
}

// @sk-test 111-provider-auth-and-config#T4.1: TestProviderConfig_AuthDefaults — defaults для auth_scheme/auth_header (AC-003)
func TestProviderConfig_AuthDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n      api_keys:\n        - sk-key\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseAndLoadConfig([]string{"--config=" + cfgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Routing.Providers[0].AuthScheme != "bearer" {
		t.Errorf("expected AuthScheme=bearer, got %q", cfg.Routing.Providers[0].AuthScheme)
	}
	if cfg.Routing.Providers[0].AuthHeader != "Authorization" {
		t.Errorf("expected AuthHeader=Authorization, got %q", cfg.Routing.Providers[0].AuthHeader)
	}
	if cfg.Routing.Providers[0].AuthPrefix != "Bearer " {
		t.Errorf("expected AuthPrefix=Bearer , got %q", cfg.Routing.Providers[0].AuthPrefix)
	}
}

// @sk-test 111-provider-auth-and-config#T4.3: TestProviderConfig_RequireAPIKeys — ошибка при отсутствии api_keys (AC-005)
func TestProviderConfig_RequireAPIKeys(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// Provider with name but no api_keys — should fail validation
	content := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseAndLoadConfig([]string{"--config=" + cfgPath})
	if err == nil {
		t.Fatal("expected error for missing api_keys, got nil")
	}
}

// @sk-test 111-provider-auth-and-config#T4.3: TestProviderConfig_RedactAPIKeys — маскировка ключей в логах (AC-006)
func TestProviderConfig_RedactAPIKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := ProviderConfig{
		Name:       "test",
		APIKeys:    []string{"secret-value"},
		AuthScheme: "bearer",
		AuthHeader: "Authorization",
	}
	logger.LogAttrs(context.Background(), slog.LevelDebug, "config", slog.Any("provider", cfg))
	output := buf.String()
	if output == "" {
		t.Fatal("expected log output, got empty")
	}
	// The original key must NOT appear in the log
	if bytes.Contains(buf.Bytes(), []byte("secret-value")) {
		t.Errorf("expected APIKey to be masked, but found 'secret-value' in log output:\n%s", output)
	}
	// The masked value should appear
	if !bytes.Contains(buf.Bytes(), []byte("****")) {
		t.Errorf("expected masked value '****' in log output:\n%s", output)
	}
}

// @sk-test config-hot-reload#T4.1: TestDiffSections_DetectsRoutingChange (AC-001)
func TestDiffSections_DetectsRoutingChange(t *testing.T) {
	old := DefaultConfig()
	old.Routing = &RoutingConfig{
		Providers: []ProviderConfig{{Name: "p1"}},
	}
	new := DefaultConfig()
	new.Routing = &RoutingConfig{
		Providers: []ProviderConfig{{Name: "p2"}},
	}
	changed := DiffSections(old, new)
	if !changed["routing"] {
		t.Error("expected routing to be changed")
	}
	if changed["tenants"] || changed["shield"] || changed["ratelimit"] || changed["debug"] {
		t.Error("expected only routing to be changed")
	}
}

// @sk-test config-hot-reload#T4.1: TestDiffSections_NoDiff (AC-006)
func TestDiffSections_NoDiff(t *testing.T) {
	old := DefaultConfig()
	new := DefaultConfig()
	changed := DiffSections(old, new)
	for section, v := range changed {
		if v {
			t.Errorf("expected no changes, but %s is marked changed", section)
		}
	}
}

// @sk-test config-hot-reload#T4.1: TestDiffSections_BaseOnlyDiffIgnored (AC-006)
func TestDiffSections_BaseOnlyDiffIgnored(t *testing.T) {
	old := DefaultConfig()
	new := DefaultConfig()
	new.Server = &ServerConfig{Port: 9999}
	new.DB = &DatabaseConfig{DSN: "postgres://changed"}
	changed := DiffSections(old, new)
	for section, v := range changed {
		if v {
			t.Errorf("expected no runtime sections changed, but %s is marked changed", section)
		}
	}
}

// @sk-test config-hot-reload#T4.1: TestWatchConfigDir_DebounceAndReload (AC-004)
func TestWatchConfigDir_DebounceAndReload(t *testing.T) {
	dir := t.TempDir()
	// Write initial valid config
	initial := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: p1\n      base_url: http://initial\n      api_type: openai\n      api_keys:\n        - k1\n")
	if err := os.WriteFile(filepath.Join(dir, "00-config-base.yaml"), []byte("log:\n  level: debug\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "99-config-runtime.yaml"), initial, 0644); err != nil {
		t.Fatal(err)
	}

	// Verify LoadConfigFromDir works
	_, err := LoadConfigFromDir(dir)
	if err != nil {
		t.Fatalf("initial load failed: %v", err)
	}

	reloaded := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	WatchConfigDir(ctx, dir, func(old, new *Config) {
		if new != nil && new.Routing != nil && len(new.Routing.Providers) > 0 &&
			new.Routing.Providers[0].Name == "p2" {
			select {
			case reloaded <- struct{}{}:
			default:
			}
		}
	})

	// Write updated config
	updated := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: p2\n      base_url: http://updated\n      api_type: openai\n      api_keys:\n        - k2\n")
	if err := os.WriteFile(filepath.Join(dir, "99-config-runtime.yaml"), updated, 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-reloaded:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for config reload")
	}
}

// @sk-test config-hot-reload#T4.1: TestConfigDirFromArgs (AC-001)
func TestConfigDirFromArgs(t *testing.T) {
	// Test CONFIG_DIR env
	t.Setenv("CONFIG_DIR", "/etc/maskchain/conf.d")
	if d := ConfigDirFromArgs(); d != "/etc/maskchain/conf.d" {
		t.Errorf("expected /etc/maskchain/conf.d from env, got %q", d)
	}
}

// @sk-test config-hot-reload#T4.1: TestConfigDirFromArgs_Flag (AC-001)
func TestConfigDirFromArgs_Flag(t *testing.T) {
	// Save and restore os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"gateway", "--config-dir", "/custom/path"}

	// Clear CONFIG_DIR env to avoid interference
	t.Setenv("CONFIG_DIR", "")

	if d := ConfigDirFromArgs(); d != "/custom/path" {
		t.Errorf("expected /custom/path from flag, got %q", d)
	}
}

// @sk-test conversation-logging#T1.1: TestConversationsConfig_EnvOverride via CONFIG_CONVERSATIONS_* (AC-008)
func TestConversationsConfig_EnvOverride(t *testing.T) {
	t.Setenv("CONFIG_CONVERSATIONS_ENABLED", "true")
	t.Setenv("CONFIG_CONVERSATIONS_RETENTION_DAYS", "7")

	dir := t.TempDir()
	content := []byte("log:\n  level: debug\nconversations:\n  enabled: false\n  retention_days: 90\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseAndLoadConfig([]string{"--config", filepath.Join(dir, "config.yaml")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Conversations == nil {
		t.Fatal("expected Conversations config, got nil")
	}
	if !cfg.Conversations.Enabled {
		t.Error("expected Conversations.Enabled=true from env")
	}
	if cfg.Conversations.RetentionDays != 7 {
		t.Errorf("expected RetentionDays=7, got %d", cfg.Conversations.RetentionDays)
	}
}

// @sk-test conversation-logging#T1.1: TestConversationsConfig_Defaults (AC-008)
func TestConversationsConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Conversations == nil {
		t.Fatal("expected Conversations defaults, got nil")
	}
	if cfg.Conversations.Enabled {
		t.Error("expected Conversations.Enabled=false by default")
	}
	if cfg.Conversations.RetentionDays == 0 {
		t.Error("expected Conversations.RetentionDays default to be non-zero")
	}
}

// @sk-test 403-key-at-rest-encryption#T1.2: TestMissingKeysKeyFailsClosed (AC-008)
func TestMissingKeysKeyFailsClosed(t *testing.T) {
	dir := t.TempDir()
	content := []byte("log:\n  level: debug\ndatabase:\n  dsn: postgres://localhost:5432/maskchain\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n      api_keys:\n        - sk-abc123\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KeysKeyEnvVar, "")
	_, err := ParseAndLoadConfig([]string{"--config", filepath.Join(dir, "config.yaml")})
	if err == nil {
		t.Fatal("expected error for missing keys key, got nil")
	}
	if !strings.Contains(err.Error(), KeysKeyEnvVar) {
		t.Errorf("expected error to mention %s, got: %v", KeysKeyEnvVar, err)
	}
}

// @sk-test 403-key-at-rest-encryption#T1.2: TestInvalidKeysKeyFailsClosed (AC-008)
func TestInvalidKeysKeyFailsClosed(t *testing.T) {
	dir := t.TempDir()
	content := []byte("log:\n  level: debug\ndatabase:\n  dsn: postgres://localhost:5432/maskchain\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n      api_keys:\n        - sk-abc123\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KeysKeyEnvVar, "not-base64")
	_, err := ParseAndLoadConfig([]string{"--config", filepath.Join(dir, "config.yaml")})
	if err == nil {
		t.Fatal("expected error for invalid keys key, got nil")
	}
}

// @sk-test 403-key-at-rest-encryption#T1.2: TestValidKeysKeyPasses (AC-008)
func TestValidKeysKeyPasses(t *testing.T) {
	dir := t.TempDir()
	content := []byte("log:\n  level: debug\ndatabase:\n  dsn: postgres://localhost:5432/maskchain\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n      api_keys:\n        - sk-abc123\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	valid := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	t.Setenv(KeysKeyEnvVar, valid)
	cfg, err := ParseAndLoadConfig([]string{"--config", filepath.Join(dir, "config.yaml")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = cfg
}

// @sk-test 403-key-at-rest-encryption#T1.2: TestNoDBKeyNotRequired (AC-008)
func TestNoDBKeyNotRequired(t *testing.T) {
	dir := t.TempDir()
	content := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n      api_keys:\n        - sk-abc123\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KeysKeyEnvVar, "")
	_, err := ParseAndLoadConfig([]string{"--config", filepath.Join(dir, "config.yaml")})
	if err != nil {
		t.Fatalf("unexpected error without DB: %v", err)
	}
}

// @sk-test 403-key-at-rest-encryption#T4.5: TenantsWithoutDBFailsClosed (AC-007)
func TestTenantsWithoutDBFailsClosed(t *testing.T) {
	dir := t.TempDir()
	content := []byte("log:\n  level: debug\ntenants:\n  acme:\n    auth_header: Authorization\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KeysKeyEnvVar, "")
	_, err := ParseAndLoadConfig([]string{"--config", filepath.Join(dir, "config.yaml")})
	if err == nil {
		t.Fatal("expected error for tenants without DB (virtual-key auth requires DB), got nil")
	}
	if !strings.Contains(err.Error(), "virtual-key auth requires DB") {
		t.Errorf("expected error to mention virtual-key auth requires DB, got: %v", err)
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

// @sk-test provider-model-registry#T3.3: wildcard route validation (AC-007)
func TestValidateRoutingRoutes(t *testing.T) {
	base := func(rules ...RuleConfig) *Config {
		return &Config{Routing: &RoutingConfig{
			Providers: []ProviderConfig{{Name: "groq", APIType: "openai", BaseURL: "http://groq", APIKeys: []string{"k"}}},
			Rules:     rules,
		}}
	}

	if err := validateRoutingRoutes(base(RuleConfig{Tenant: "*", Routes: []RouteConfig{{Model: "groq/*", Providers: []string{"groq"}}}})); err != nil {
		t.Errorf("valid wildcard rejected: %v", err)
	}
	if err := validateRoutingRoutes(base(RuleConfig{Tenant: "*", Routes: []RouteConfig{{Model: "foo*bar", Providers: []string{"groq"}}}})); err == nil {
		t.Error("malformed wildcard pattern must be rejected")
	}
	if err := validateRoutingRoutes(base(RuleConfig{Tenant: "*", Routes: []RouteConfig{{Model: "groq/*", Providers: []string{"nope"}}}})); err == nil {
		t.Error("wildcard referencing an unknown provider must be rejected")
	}
	if err := validateRoutingRoutes(base(RuleConfig{Tenant: "*", Routes: []RouteConfig{{Model: "gpt-4o", Providers: []string{"nope"}}}})); err != nil {
		t.Errorf("non-wildcard routes are not validated here: %v", err)
	}
}

// @sk-test model-aliases-weighted-lb#T1.6: provider weight and routing aliases parse from YAML (AC-009)
func TestRoutingConfigWeightAndAliases(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("log:\n  level: debug\nrouting:\n  providers:\n    - name: test\n      base_url: https://api.example.com/v1\n      api_type: openai\n      api_keys:\n        - sk-x\n      weight: 3\n  aliases:\n    - tenant: acme\n      alias: gpt-4o\n      target: openai/gpt-4o-2024\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseAndLoadConfig([]string{"--config=" + cfgPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Routing.Providers[0].Weight != 3 {
		t.Errorf("weight = %d, want 3", cfg.Routing.Providers[0].Weight)
	}
	if len(cfg.Routing.Aliases) != 1 {
		t.Fatalf("aliases = %+v, want 1", cfg.Routing.Aliases)
	}
	if a := cfg.Routing.Aliases[0]; a.Tenant != "acme" || a.Alias != "gpt-4o" || a.Target != "openai/gpt-4o-2024" {
		t.Errorf("alias = %+v, want acme/gpt-4o -> openai/gpt-4o-2024", a)
	}
}

// @sk-test model-aliases-weighted-lb#T3.3: negative weight and empty alias target are rejected (AC-010)
func TestValidateRoutingWeightAndAliases(t *testing.T) {
	neg := &Config{Routing: &RoutingConfig{Providers: []ProviderConfig{{Name: "p", Weight: -1}}}}
	if err := validateRoutingWeightAndAliases(neg); err == nil {
		t.Error("negative weight must be rejected")
	}

	emptyTarget := &Config{Routing: &RoutingConfig{Aliases: []AliasConfig{{Tenant: "acme", Alias: "m", Target: ""}}}}
	if err := validateRoutingWeightAndAliases(emptyTarget); err == nil {
		t.Error("empty alias target must be rejected")
	}

	valid := &Config{Routing: &RoutingConfig{
		Providers: []ProviderConfig{{Name: "p", Weight: 2}},
		Aliases:   []AliasConfig{{Tenant: "acme", Alias: "m", Target: "t"}},
	}}
	if err := validateRoutingWeightAndAliases(valid); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
}
