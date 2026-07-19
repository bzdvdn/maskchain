package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// envVarReplacer maps viper key format to env var format (reverse of SetEnvKeyReplacer).
// Used to pre-populate viper from CONFIG_* env vars for Unmarshal to pick up.
var envToKeyReplacer = strings.NewReplacer("_", ".", "-", ".")

// @sk-task 01-config-bootstrap#T1.2: Create Config struct with LogConfig, mapstructure/yaml/validate tags, defaults (AC-001, AC-003)
type LogConfig struct {
	Level string `mapstructure:"level" yaml:"level" validate:"required"`
}

type ServerConfig struct {
	Port                 int                `mapstructure:"port" yaml:"port"`
	AdminPort            int                `mapstructure:"admin_port" yaml:"admin_port"`
	ShutdownTimeout      int                `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	CORSOrigins          []string           `mapstructure:"cors_origins" yaml:"cors_origins"`
	HealthCheck          *HealthCheckConfig `mapstructure:"health_check" yaml:"health_check"`
	TenantReloadInterval time.Duration      `mapstructure:"tenant_reload_interval" yaml:"tenant_reload_interval"`
}

// @sk-task 114-real-health-probes#T1.2: Add HealthCheckConfig with CriticalDeps (AC-006)
type HealthCheckConfig struct {
	CriticalDeps []string `mapstructure:"critical_deps" yaml:"critical_deps"`
}

// @sk-task 22-shield-mask-storage#T5.1: Add Database/Valkey/Mask config (AC-all)
// @sk-task 30-shield-persistence#T1.3: Add pool params to DatabaseConfig (AC-005)
type DatabaseConfig struct {
	DSN             string        `mapstructure:"dsn" yaml:"dsn"`
	MaxConns        int           `mapstructure:"max_conns" yaml:"max_conns"`
	MinConns        int           `mapstructure:"min_conns" yaml:"min_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime" yaml:"max_conn_lifetime"`
}

// @sk-task 22-shield-mask-storage#T5.1: Add Valkey config section (AC-all)
type ValkeyConfig struct {
	Addr     string `mapstructure:"addr" yaml:"addr"`
	Password string `mapstructure:"password" yaml:"password"`
	TTLSec   int    `mapstructure:"ttl_sec" yaml:"ttl_sec"`
}

// @sk-task 22-shield-mask-storage#T5.1: Add Mask config section (AC-all)
type MaskConfig struct {
	CacheTTLSec int `mapstructure:"cache_ttl_sec" yaml:"cache_ttl_sec"`
}

// @sk-task sessions#T1.3: Add SessionConfig section (AC-001, AC-002, AC-005, AC-007, AC-010)
type SessionConfig struct {
	DefaultTTL      time.Duration `mapstructure:"default_ttl" yaml:"default_ttl"`
	MaxTTL          time.Duration `mapstructure:"max_ttl" yaml:"max_ttl"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval" yaml:"cleanup_interval"`
	CleanupEnabled  bool          `mapstructure:"cleanup_enabled" yaml:"cleanup_enabled"`
	CacheTTL        time.Duration `mapstructure:"cache_ttl" yaml:"cache_ttl"`
}

// @sk-task 51-shield-gateway-integration#T1.1: Add ShieldConfig section (AC-001, AC-002)
// @sk-task 13-shield-middleware-wiring#T2.2: Remove ProfileMapping and DefaultAction (AC-005)
type ShieldConfig struct {
	ActionOnSuspicious string                       `mapstructure:"action_on_suspicious" yaml:"action_on_suspicious"`
	TenantModelMapping map[string]map[string]string `mapstructure:"tenant_model_mapping" yaml:"tenant_model_mapping"`
}

// @sk-task 70-routing-engine#T1.2: Add routing config structs (AC-001, AC-002, AC-005)
// @sk-task 110-provider-adapters#T1.1: Add APIType and APIKey fields (AC-007, AC-008)
// @sk-task 111-provider-auth-and-config#T1.1: Add APIKeys, AuthScheme, AuthHeader, AdditionalHeaders (AC-003)
// @sk-task provider-adapters-expansion#T1.1: Add AWS Bedrock config fields (AC-004)
type ProviderConfig struct {
	Name              string            `mapstructure:"name" yaml:"name"`
	BaseURL           string            `mapstructure:"base_url" yaml:"base_url"`
	HealthEndpoint    string            `mapstructure:"health_endpoint" yaml:"health_endpoint"`
	Timeout           string            `mapstructure:"timeout" yaml:"timeout"`
	Priority          int               `mapstructure:"priority" yaml:"priority"`
	APIType           string            `mapstructure:"api_type" yaml:"api_type"`
	APIKeys           []string          `mapstructure:"api_keys" yaml:"api_keys" validate:"required"`
	AuthScheme        string            `mapstructure:"auth_scheme" yaml:"auth_scheme"`
	AuthHeader        string            `mapstructure:"auth_header" yaml:"auth_header"`
	AuthPrefix        string            `mapstructure:"auth_prefix" yaml:"auth_prefix"`
	AdditionalHeaders map[string]string `mapstructure:"additional_headers" yaml:"additional_headers"`
	ProxyURL          string            `mapstructure:"proxy_url" yaml:"proxy_url"`
	AWSRegion         string            `mapstructure:"aws_region" yaml:"aws_region"`
	AWSAccessKeyID    string            `mapstructure:"aws_access_key_id" yaml:"aws_access_key_id"`
	AWSSecretAccessKey string           `mapstructure:"aws_secret_access_key" yaml:"aws_secret_access_key"`
}

type RouteConfig struct {
	Model     string   `mapstructure:"model" yaml:"model"`
	Providers []string `mapstructure:"providers" yaml:"providers"`
}

type RuleConfig struct {
	Tenant string        `mapstructure:"tenant" yaml:"tenant"`
	Routes []RouteConfig `mapstructure:"routes" yaml:"routes"`
}

type RoutingConfig struct {
	Providers []ProviderConfig `mapstructure:"providers" yaml:"providers"`
	Rules     []RuleConfig     `mapstructure:"rules" yaml:"rules"`
}

// @sk-task 61-observability#T1.2: Add OtelConfig section (AC-001, AC-006, AC-007)
type OtelConfig struct {
	Endpoint      string  `mapstructure:"endpoint" yaml:"endpoint"`
	ServiceName   string  `mapstructure:"service_name" yaml:"service_name"`
	Environment   string  `mapstructure:"environment" yaml:"environment"`
	SamplingRatio float64 `mapstructure:"sampling_ratio" yaml:"sampling_ratio"`
}

// @sk-task admin-ui-design#T1.1: Add AdminConfig for env-based admin auth (AC-001, AC-004)
type AdminConfig struct {
	Username              string        `mapstructure:"username" yaml:"username"`
	Password              string        `mapstructure:"password" yaml:"password"`
	SessionTTL            time.Duration `mapstructure:"session_ttl" yaml:"session_ttl"`
	DashboardPollInterval time.Duration `mapstructure:"dashboard_poll_interval" yaml:"dashboard_poll_interval"`
}

// @sk-task 80-tenant-isolation#T1.2: Add TenantConfig struct (AC-001, AC-003, AC-004)
type TenantConfig struct {
	Name       string            `mapstructure:"name" yaml:"name"`
	AuthHeader string            `mapstructure:"auth_header" yaml:"auth_header"`
	AuthScheme string            `mapstructure:"auth_scheme" yaml:"auth_scheme"`
	APIKeys    []string          `mapstructure:"api_keys" yaml:"api_keys" validate:"required"`
	PIIConfig  *entity.PIIConfig `mapstructure:"pii_config" yaml:"pii_config"`
}

// @sk-task rate-limiting-budgets#T1.1: Add RateLimitConfig with defaults (AC-006)
type RateLimitConfig struct {
	DefaultRatePerWindow int                           `mapstructure:"default_rate_per_window" yaml:"default_rate_per_window"`
	DefaultWindowSec     int                           `mapstructure:"default_window_sec" yaml:"default_window_sec"`
	DefaultTokenBudget   map[string]int64              `mapstructure:"default_token_budget" yaml:"default_token_budget"`
	TenantOverrides      map[string]*RateLimitOverride `mapstructure:"tenant_overrides" yaml:"tenant_overrides"`
}

type RateLimitOverride struct {
	RatePerWindow *int             `mapstructure:"rate_per_window" yaml:"rate_per_window"`
	WindowSec     *int             `mapstructure:"window_sec" yaml:"window_sec"`
	TokenBudget   map[string]int64 `mapstructure:"token_budget" yaml:"token_budget"`
}

// @sk-task 71-egress-streaming#T1.2: Add EgressConfig section (AC-002, AC-004, AC-006, AC-007)
// @sk-task 90-production-hardening#T1.2: Add MaxIdleConnsPerHost and DisableKeepAlives (<AC-002>)
// @sk-task 116-connection-pool-fixes#T1.1: Add TLS and CircuitBreaker to EgressConfig (AC-003, AC-004, AC-005, AC-006, AC-007)
type EgressConfig struct {
	MaxIdleConns        int                   `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	IdleTimeout         time.Duration         `mapstructure:"idle_timeout" yaml:"idle_timeout"`
	MaxRetries          int                   `mapstructure:"max_retries" yaml:"max_retries"`
	BaseBackoff         time.Duration         `mapstructure:"base_backoff" yaml:"base_backoff"`
	RetryOn5xx          bool                  `mapstructure:"retry_on_5xx" yaml:"retry_on_5xx"`
	MaxIdleConnsPerHost int                   `mapstructure:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	DisableKeepAlives   bool                  `mapstructure:"disable_keep_alives" yaml:"disable_keep_alives"`
	TLS                 *EgressTLSConfig      `mapstructure:"tls" yaml:"tls"`
	CircuitBreaker      *CircuitBreakerConfig `mapstructure:"circuit_breaker" yaml:"circuit_breaker"`
	DebugEnabled        bool                  `mapstructure:"debug_enabled" yaml:"debug_enabled"`
}

// @sk-task 116-connection-pool-fixes#T1.1: Add EgressTLSConfig struct (AC-003, AC-004, AC-005)
type EgressTLSConfig struct {
	CACert             string `mapstructure:"ca_cert" yaml:"ca_cert"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	Cert               string `mapstructure:"cert" yaml:"cert"`
	Key                string `mapstructure:"key" yaml:"key"`
}

// @sk-task 116-connection-pool-fixes#T1.1: Add CircuitBreakerConfig struct (AC-006, AC-007)
type CircuitBreakerConfig struct {
	MaxFailures int           `mapstructure:"max_failures" yaml:"max_failures"`
	Cooldown    time.Duration `mapstructure:"cooldown" yaml:"cooldown"`
}

// @sk-task 90-production-hardening#T1.1: Add DebugConfig struct (<AC-001>)
type DebugConfig struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled"`
	AdminToken string `mapstructure:"admin_token" yaml:"admin_token"`
}

// @sk-task 102-profile-cache#T1.2: Add DictionaryCacheConfig struct (RQ-009, RQ-010)
type DictionaryCacheConfig struct {
	ValkeyTTLSec    int  `mapstructure:"valkey_ttl_sec" yaml:"valkey_ttl_sec"`
	LRUSize         int  `mapstructure:"lru_size" yaml:"lru_size"`
	WarmOnStartup   bool `mapstructure:"warm_on_startup" yaml:"warm_on_startup"`
	WarmConcurrency int  `mapstructure:"warm_concurrency" yaml:"warm_concurrency"`
}

// @sk-task 131-analytics-pipeline#T1.2: Add AnalyticsConfig with CostRateConfig (AC-007)
type CostRateConfig struct {
	Model            string  `mapstructure:"model" yaml:"model"`
	InputPricePer1K  float64 `mapstructure:"input_price_per_1k" yaml:"input_price_per_1k"`
	OutputPricePer1K float64 `mapstructure:"output_price_per_1k" yaml:"output_price_per_1k"`
}

// @sk-task 131-analytics-pipeline#T1.2: Add AnalyticsConfig (AC-002, AC-007, AC-008)
type AnalyticsConfig struct {
	CostRates     []CostRateConfig `mapstructure:"cost_rates" yaml:"cost_rates"`
	RetentionDays int              `mapstructure:"retention_days" yaml:"retention_days"`
	BatchInterval string           `mapstructure:"batch_interval" yaml:"batch_interval"`
}

// @sk-task 80-tenant-isolation#T1.2: Add Tenants map to Config struct (AC-001, AC-003, AC-004, AC-005)
// @sk-task 90-production-hardening#T1.1: Wire Debug into Config (<AC-001>)
type Config struct {
	Log             *LogConfig               `mapstructure:"log" yaml:"log"`
	Server          *ServerConfig            `mapstructure:"server" yaml:"server"`
	DB              *DatabaseConfig          `mapstructure:"database" yaml:"database"`
	Valkey          *ValkeyConfig            `mapstructure:"valkey" yaml:"valkey"`
	Mask            *MaskConfig              `mapstructure:"mask" yaml:"mask"`
	Shield          *ShieldConfig            `mapstructure:"shield" yaml:"shield"`
	Routing         *RoutingConfig           `mapstructure:"routing" yaml:"routing"`
	OTel            *OtelConfig              `mapstructure:"otel" yaml:"otel"`
	RateLimit       *RateLimitConfig         `mapstructure:"ratelimit" yaml:"ratelimit"`
	Egress          *EgressConfig            `mapstructure:"egress" yaml:"egress"`
	Debug           *DebugConfig             `mapstructure:"debug" yaml:"debug"`
	Session         *SessionConfig           `mapstructure:"session" yaml:"session"`
	DictionaryCache *DictionaryCacheConfig   `mapstructure:"dictionary_cache" yaml:"dictionary_cache"`
	Analytics       *AnalyticsConfig         `mapstructure:"analytics" yaml:"analytics"`
	Tenants         map[string]*TenantConfig `mapstructure:"tenants" yaml:"tenants"`
	Admin           *AdminConfig             `mapstructure:"admin" yaml:"admin"`
}

var _ zapcore.ObjectMarshaler = (*Config)(nil)

// LoadConfigFromDir reads all *.yaml files from dir, deep-merges them in
// alphabetical order (last file wins), unmarshals into a Config, and applies
// defaults + normalisation + validation.
func LoadConfigFromDir(dir string) (*Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read config dir %q: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ext := filepath.Ext(e.Name()); ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		return nil, fmt.Errorf("no yaml files found in config dir %q", dir)
	}

	merged := make(map[string]interface{})
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read config file %q: %w", f, err)
		}
		expanded := os.ExpandEnv(string(data))
		var m map[string]interface{}
		if err := yaml.Unmarshal([]byte(expanded), &m); err != nil {
			return nil, fmt.Errorf("parse config file %q: %w", f, err)
		}
		deepMergeMaps(merged, m)
	}

	cfg := DefaultConfig()
	v := viper.New()
	if err := v.MergeConfigMap(merged); err != nil {
		return nil, fmt.Errorf("merge config map: %w", err)
	}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	normalizeProviderConfig(cfg, v)

	if err := validateConfig(cfg, v); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadConfig reads configuration from a single file, a directory of YAML files,
// or env vars. Precedence: --config-dir > CONFIG_DIR > --config > CONFIG_FILE_PATH > default (config.yaml).
func LoadConfig(cmd *cobra.Command) (*Config, error) {
	cfgDir, _ := cmd.Flags().GetString("config-dir")
	cfgPath, _ := cmd.Flags().GetString("config")

	// --config-dir or CONFIG_DIR env var → directory mode
	if cfgDir == "" {
		cfgDir = os.Getenv("CONFIG_DIR")
	}
	if cfgDir != "" {
		return LoadConfigFromDir(cfgDir)
	}

	// --config flag not explicitly changed → check CONFIG_FILE_PATH env var
	if !cmd.Flags().Changed("config") {
		if envPath := os.Getenv("CONFIG_FILE_PATH"); envPath != "" {
			cfgPath = envPath
		}
	}

	v := viper.New()

	if cmd.Flags().Changed("config") {
		v.SetConfigFile(cfgPath)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("CONFIG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.BindPFlags(cmd.Flags()); err != nil {
		return nil, fmt.Errorf("bind flags: %w", err)
	}

	// Map CLI flag --log-level to nested viper key log.level
	if err := v.BindPFlag("log.level", cmd.Flags().Lookup("log-level")); err != nil {
		return nil, fmt.Errorf("bind log-level flag: %w", err)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	} else if cfgFile := v.ConfigFileUsed(); cfgFile != "" {
		data, err := os.ReadFile(cfgFile)
		if err == nil {
			expanded := os.ExpandEnv(string(data))
			if err := v.ReadConfig(bytes.NewReader([]byte(expanded))); err != nil {
				return nil, fmt.Errorf("read expanded config: %w", err)
			}
		}
	}

	// Pre-populate from CONFIG_* env vars so Unmarshal picks them up
	// even without a config file (AutomaticEnv only works with Get()).
	// Multi-line values (YAML blocks) are skipped — they require a config file.
	prefix := "CONFIG_"
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, prefix) {
			continue
		}
		eq := strings.IndexByte(e, '=')
		if eq < 0 {
			continue
		}
		envName := e[:eq]
		val := e[eq+1:]
		key := strings.TrimPrefix(envName, prefix)
		key = strings.ToLower(envToKeyReplacer.Replace(key))
		if key == "" || strings.Contains(val, "\n") {
			continue
		}
		v.SetDefault(key, val)
	}

	cfg := DefaultConfig()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	normalizeProviderConfig(cfg, v)

	if err := validateConfig(cfg, v); err != nil {
		return nil, err
	}

	return cfg, nil
}
