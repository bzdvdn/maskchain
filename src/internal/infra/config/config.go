package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"

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
	Port                int                `mapstructure:"port" yaml:"port"`
	ShutdownTimeout     int                `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	CORSOrigins         []string           `mapstructure:"cors_origins" yaml:"cors_origins"`
	HealthCheck         *HealthCheckConfig `mapstructure:"health_check" yaml:"health_check"`
	TenantReloadInterval time.Duration     `mapstructure:"tenant_reload_interval" yaml:"tenant_reload_interval"`
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
	DefaultTTL       time.Duration `mapstructure:"default_ttl" yaml:"default_ttl"`
	MaxTTL           time.Duration `mapstructure:"max_ttl" yaml:"max_ttl"`
	CleanupInterval  time.Duration `mapstructure:"cleanup_interval" yaml:"cleanup_interval"`
	CleanupEnabled   bool          `mapstructure:"cleanup_enabled" yaml:"cleanup_enabled"`
	CacheTTL         time.Duration `mapstructure:"cache_ttl" yaml:"cache_ttl"`
}

// @sk-task 51-shield-gateway-integration#T1.1: Add ShieldConfig section (AC-001, AC-002)
// @sk-task 13-shield-middleware-wiring#T2.2: Remove ProfileMapping and DefaultAction (AC-005)
type ShieldConfig struct {
	ActionOnSuspicious string                       `mapstructure:"action_on_suspicious" yaml:"action_on_suspicious"`
	TenantModelMapping map[string]map[string]string  `mapstructure:"tenant_model_mapping" yaml:"tenant_model_mapping"`
}

// @sk-task 70-routing-engine#T1.2: Add routing config structs (AC-001, AC-002, AC-005)
// @sk-task 110-provider-adapters#T1.1: Add APIType and APIKey fields (AC-007, AC-008)
// @sk-task 111-provider-auth-and-config#T1.1: Add APIKeys, AuthScheme, AuthHeader, AdditionalHeaders (AC-003)
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
	Name        string                `mapstructure:"name" yaml:"name"`
	ProfileSlug string                `mapstructure:"profile_slug" yaml:"profile_slug"`
	AuthHeader  string                `mapstructure:"auth_header" yaml:"auth_header"`
	AuthScheme  string                `mapstructure:"auth_scheme" yaml:"auth_scheme"`
	APIKeys     []string              `mapstructure:"api_keys" yaml:"api_keys" validate:"required"`
	PIIConfig   *entity.PIIConfig     `mapstructure:"pii_config" yaml:"pii_config"`
}

// @sk-task rate-limiting-budgets#T1.1: Add RateLimitConfig with defaults (AC-006)
type RateLimitConfig struct {
	DefaultRatePerWindow int                           `mapstructure:"default_rate_per_window" yaml:"default_rate_per_window"`
	DefaultWindowSec     int                           `mapstructure:"default_window_sec" yaml:"default_window_sec"`
	DefaultTokenBudget   map[string]int64              `mapstructure:"default_token_budget" yaml:"default_token_budget"`
	TenantOverrides      map[string]*RateLimitOverride `mapstructure:"tenant_overrides" yaml:"tenant_overrides"`
}

type RateLimitOverride struct {
	RatePerWindow *int              `mapstructure:"rate_per_window" yaml:"rate_per_window"`
	WindowSec     *int              `mapstructure:"window_sec" yaml:"window_sec"`
	TokenBudget   map[string]int64  `mapstructure:"token_budget" yaml:"token_budget"`
}

// @sk-task 71-egress-streaming#T1.2: Add EgressConfig section (AC-002, AC-004, AC-006, AC-007)
// @sk-task 90-production-hardening#T1.2: Add MaxIdleConnsPerHost and DisableKeepAlives (<AC-002>)
// @sk-task 116-connection-pool-fixes#T1.1: Add TLS and CircuitBreaker to EgressConfig (AC-003, AC-004, AC-005, AC-006, AC-007)
type EgressConfig struct {
	MaxIdleConns        int                    `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	IdleTimeout         time.Duration          `mapstructure:"idle_timeout" yaml:"idle_timeout"`
	MaxRetries          int                    `mapstructure:"max_retries" yaml:"max_retries"`
	BaseBackoff         time.Duration          `mapstructure:"base_backoff" yaml:"base_backoff"`
	RetryOn5xx          bool                   `mapstructure:"retry_on_5xx" yaml:"retry_on_5xx"`
	MaxIdleConnsPerHost int                    `mapstructure:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	DisableKeepAlives   bool                   `mapstructure:"disable_keep_alives" yaml:"disable_keep_alives"`
	TLS                 *EgressTLSConfig       `mapstructure:"tls" yaml:"tls"`
	CircuitBreaker      *CircuitBreakerConfig  `mapstructure:"circuit_breaker" yaml:"circuit_breaker"`
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
	Log    *LogConfig    `mapstructure:"log" yaml:"log"`
	Server *ServerConfig `mapstructure:"server" yaml:"server"`
	DB     *DatabaseConfig `mapstructure:"database" yaml:"database"`
	Valkey *ValkeyConfig   `mapstructure:"valkey" yaml:"valkey"`
	Mask   *MaskConfig     `mapstructure:"mask" yaml:"mask"`
	Shield *ShieldConfig   `mapstructure:"shield" yaml:"shield"`
	Routing *RoutingConfig `mapstructure:"routing" yaml:"routing"`
	OTel   *OtelConfig     `mapstructure:"otel" yaml:"otel"`
	RateLimit *RateLimitConfig `mapstructure:"ratelimit" yaml:"ratelimit"`
	Egress    *EgressConfig    `mapstructure:"egress" yaml:"egress"`
	Debug     *DebugConfig     `mapstructure:"debug" yaml:"debug"`
	Session         *SessionConfig         `mapstructure:"session" yaml:"session"`
	DictionaryCache *DictionaryCacheConfig `mapstructure:"dictionary_cache" yaml:"dictionary_cache"`
	Analytics       *AnalyticsConfig       `mapstructure:"analytics" yaml:"analytics"`
	Tenants         map[string]*TenantConfig `mapstructure:"tenants" yaml:"tenants"`
	Admin           *AdminConfig             `mapstructure:"admin" yaml:"admin"`
}

var _ zapcore.ObjectMarshaler = (*Config)(nil)

func (c *Config) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("log_level", c.Log.Level)
	if c.Server != nil {
		enc.AddInt("port", c.Server.Port)
		enc.AddInt("shutdown_timeout", c.Server.ShutdownTimeout)
		enc.AddString("tenant_reload_interval", c.Server.TenantReloadInterval.String())
	}
	if c.Valkey != nil && c.Valkey.Addr != "" {
		enc.AddString("valkey_addr", c.Valkey.Addr)
		enc.AddString("valkey_password", "****")
		enc.AddInt("valkey_ttl_sec", c.Valkey.TTLSec)
	}
	if c.Routing != nil {
		_ = enc.AddArray("providers", zapcore.ArrayMarshalerFunc(func(aenc zapcore.ArrayEncoder) error {
			for _, p := range c.Routing.Providers {
				_ = aenc.AppendObject(providerLogEntryFromConfig(p))
			}
			return nil
		}))
	}
	if c.OTel != nil {
		enc.AddString("otel_endpoint", c.OTel.Endpoint)
	}
	if c.Tenants != nil {
		_ = enc.AddArray("tenants", zapcore.ArrayMarshalerFunc(func(aenc zapcore.ArrayEncoder) error {
			for slug := range c.Tenants {
				aenc.AppendString(slug)
			}
			return nil
		}))
	}
	return nil
}

type providerLogEntry struct {
	Name       string
	BaseURL    string
	APIType    string
	AuthScheme string
	AuthPrefix string
}

func (p providerLogEntry) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", p.Name)
	enc.AddString("base_url", p.BaseURL)
	enc.AddString("api_type", p.APIType)
	enc.AddString("api_keys", "****")
	enc.AddString("auth_scheme", p.AuthScheme)
	enc.AddString("auth_prefix", p.AuthPrefix)
	return nil
}

func providerLogEntryFromConfig(p ProviderConfig) providerLogEntry {
	return providerLogEntry{
		Name:       p.Name,
		BaseURL:    p.BaseURL,
		APIType:    p.APIType,
		AuthScheme: p.AuthScheme,
		AuthPrefix: p.AuthPrefix,
	}
}

const defaultLogLevel = "info"
const defaultPort = 8080
const defaultShutdownTimeout = 30
const defaultValkeyAddr = "localhost:6379"
const defaultValkeyTTL = 3600
const defaultMaskCacheTTL = 3600
const defaultMaxDBConns = 25
const defaultMinDBConns = 1
const defaultMaxDBConnLifetimeMinutes = 30
const defaultShieldActionOnSuspicious = "block"
const defaultOtelEndpoint = "localhost:4317"
const defaultOtelServiceName = "maskchain-gateway"
const defaultOtelEnvironment = "development"
const defaultOtelSamplingRatio = 1.0
const defaultEgressMaxIdleConns = 10
const defaultEgressIdleTimeout = 30 * time.Second
const defaultEgressMaxRetries = 3
const defaultEgressBaseBackoff = 100 * time.Millisecond
const defaultEgressRetryOn5xx = false
const defaultEgressMaxIdleConnsPerHost = 2
const defaultEgressDisableKeepAlives = false
const defaultEgressCBMaxFailures = 3
const defaultEgressCBCooldown = 30 * time.Second
const defaultDebugEnabled = false
const defaultDebugAdminToken = ""
const defaultRateLimitRate = 100
const defaultRateLimitWindowSec = 60
const defaultDictionaryCacheValkeyTTL = 300
const defaultDictionaryCacheLRUSize = 10000
const defaultDictionaryCacheWarmOnStartup = true
const defaultDictionaryCacheWarmConcurrency = 5
const defaultSessionTTL = 30 * time.Minute
const defaultSessionMaxTTL = 24 * time.Hour
const defaultSessionCleanupInterval = 5 * time.Minute
const defaultSessionCleanupEnabled = false
const defaultSessionCacheTTL = 5 * time.Minute
const defaultAnalyticsRetentionDays = 7
const defaultAnalyticsBatchInterval = "5s"
const defaultHealthCheckCriticalDeps = "database"
const defaultTenantReloadInterval = 15 * time.Second
const defaultAdminSessionTTL = 30 * time.Minute
const defaultDashboardPollInterval = 5 * time.Second

// @sk-task 10-gateway-skeleton#T1.2: Set ServerConfig defaults in DefaultConfig (AC-001, AC-005)
func DefaultConfig() *Config {
	return &Config{
		Log: &LogConfig{
			Level: defaultLogLevel,
		},
		Server: &ServerConfig{
			Port:                 defaultPort,
			ShutdownTimeout:      defaultShutdownTimeout,
			TenantReloadInterval: defaultTenantReloadInterval,
			HealthCheck: &HealthCheckConfig{
				CriticalDeps: []string{defaultHealthCheckCriticalDeps},
			},
		},
		DB: &DatabaseConfig{
			MaxConns:        defaultMaxDBConns,
			MinConns:        defaultMinDBConns,
			MaxConnLifetime: time.Duration(defaultMaxDBConnLifetimeMinutes) * time.Minute,
		},
		Valkey: &ValkeyConfig{
			Addr:   defaultValkeyAddr,
			TTLSec: defaultValkeyTTL,
		},
		Mask: &MaskConfig{
			CacheTTLSec: defaultMaskCacheTTL,
		},
		Shield: &ShieldConfig{
			ActionOnSuspicious: defaultShieldActionOnSuspicious,
		},
		OTel: &OtelConfig{
			Endpoint:      defaultOtelEndpoint,
			ServiceName:   defaultOtelServiceName,
			Environment:   defaultOtelEnvironment,
			SamplingRatio: defaultOtelSamplingRatio,
		},
		Egress: &EgressConfig{
			MaxIdleConns:        defaultEgressMaxIdleConns,
			IdleTimeout:         defaultEgressIdleTimeout,
			MaxRetries:          defaultEgressMaxRetries,
			BaseBackoff:         defaultEgressBaseBackoff,
			RetryOn5xx:          defaultEgressRetryOn5xx,
			MaxIdleConnsPerHost: defaultEgressMaxIdleConnsPerHost,
			DisableKeepAlives:   defaultEgressDisableKeepAlives,
			CircuitBreaker: &CircuitBreakerConfig{
				MaxFailures: defaultEgressCBMaxFailures,
				Cooldown:    defaultEgressCBCooldown,
			},
		},
		Debug: &DebugConfig{
			Enabled:    defaultDebugEnabled,
			AdminToken: defaultDebugAdminToken,
		},
		Session: &SessionConfig{
			DefaultTTL:       defaultSessionTTL,
			MaxTTL:           defaultSessionMaxTTL,
			CleanupInterval:  defaultSessionCleanupInterval,
			CleanupEnabled:   defaultSessionCleanupEnabled,
			CacheTTL:         defaultSessionCacheTTL,
		},
		DictionaryCache: &DictionaryCacheConfig{
			ValkeyTTLSec:    defaultDictionaryCacheValkeyTTL,
			LRUSize:         defaultDictionaryCacheLRUSize,
			WarmOnStartup:   defaultDictionaryCacheWarmOnStartup,
			WarmConcurrency: defaultDictionaryCacheWarmConcurrency,
		},
		Analytics: &AnalyticsConfig{
			RetentionDays: defaultAnalyticsRetentionDays,
			BatchInterval: defaultAnalyticsBatchInterval,
		},
		Admin: &AdminConfig{
			SessionTTL:            defaultAdminSessionTTL,
			DashboardPollInterval: defaultDashboardPollInterval,
		},
	}
}

// @sk-task 01-config-bootstrap#T2.1: Implement LoadConfig with cobra root command, viper YAML/ENV/flags binding, required validation (AC-001, AC-002, AC-003, AC-005)
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "MaskChain AI Gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.Flags().String("config", "config.yaml", "path to config file")
	cmd.Flags().String("log-level", "", "log level (debug, info, warn, error)")
	return cmd
}

func LoadConfig(cmd *cobra.Command) (*Config, error) {
	cfgPath, _ := cmd.Flags().GetString("config")

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

func ParseAndLoadConfig(args []string) (*Config, error) {
	cmd := NewRootCmd()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	return LoadConfig(cmd)
}

func MustLoadConfig() *Config {
	cfg, err := ParseAndLoadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

// @sk-task 111-provider-auth-and-config#T1.2: Fallback for old api_key + apply defaults (AC-003)
func normalizeProviderConfig(cfg *Config, v *viper.Viper) {
	if cfg.Routing == nil {
		return
	}
	for i := range cfg.Routing.Providers {
		p := &cfg.Routing.Providers[i]
		if len(p.APIKeys) == 0 {
			oldKey := v.GetString(fmt.Sprintf("routing.providers.%d.api_key", i))
			if oldKey != "" {
				p.APIKeys = []string{oldKey}
			}
		}
		if p.AuthScheme == "" {
			p.AuthScheme = "bearer"
		}
		if p.AuthHeader == "" {
			p.AuthHeader = "Authorization"
		}
		if p.AuthPrefix == "" {
			switch p.AuthScheme {
			case "bearer":
				p.AuthPrefix = "Bearer "
			case "basic":
				p.AuthPrefix = "Basic "
			default:
				p.AuthPrefix = ""
			}
		}
	}
}

// @sk-task 111-provider-auth-and-config#T2.1: Validate APIKeys required + auth_scheme enum (AC-005)
// @sk-task ollama-provider#T1.1: Relax api_keys validation for ollama (AC-001)
func validateProviderAuth(cfg *Config) error {
	if cfg.Routing == nil {
		return nil
	}
	for i, p := range cfg.Routing.Providers {
		if p.Name == "" {
			continue
		}
		if len(p.APIKeys) == 0 {
			if p.APIType == "ollama" {
				continue
			}
			return fmt.Errorf("routing.providers.%d.api_keys: required for provider %q", i, p.Name)
		}
		switch p.AuthScheme {
		case "bearer", "api-key", "basic":
		default:
			return fmt.Errorf("routing.providers.%d.auth_scheme: unsupported %q (must be bearer, api-key, or basic)", i, p.AuthScheme)
		}
		if p.AuthScheme != "bearer" && p.AuthPrefix == "" {
			p.AuthPrefix = ""
		}
	}
	return nil
}

// @sk-task 111-provider-auth-and-config#T2.2: Mask APIKeys via slog.LogValuer (AC-006)
func (p ProviderConfig) LogValue() slog.Value {
	masked := make([]string, len(p.APIKeys))
	for i := range p.APIKeys {
		masked[i] = "****"
	}
	return slog.GroupValue(
		slog.String("name", p.Name),
		slog.String("base_url", p.BaseURL),
		slog.String("health_endpoint", p.HealthEndpoint),
		slog.String("timeout", p.Timeout),
		slog.Int("priority", p.Priority),
		slog.String("api_type", p.APIType),
		slog.Any("api_keys", masked),
		slog.String("auth_scheme", p.AuthScheme),
		slog.String("auth_header", p.AuthHeader),
		slog.String("auth_prefix", p.AuthPrefix),
		slog.Any("additional_headers", p.AdditionalHeaders),
	)
}

func validateConfig(cfg *Config, v *viper.Viper) error {
	if err := validateProviderAuth(cfg); err != nil {
		return err
	}
	val := reflect.ValueOf(cfg).Elem()
	t := val.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		sub := val.Field(i)
		if sub.Kind() == reflect.Ptr && !sub.IsNil() {
			prefix := field.Tag.Get("mapstructure")
			if prefix == "" {
				prefix = strings.ToLower(field.Name)
			}
			if err := validateRequiredFields(sub.Elem(), v, prefix); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRequiredFields(val reflect.Value, v *viper.Viper, prefix string) error {
	t := val.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		validateTag := field.Tag.Get("validate")
		if !strings.Contains(validateTag, "required") {
			continue
		}
		mapKey := field.Tag.Get("mapstructure")
		if mapKey == "" {
			mapKey = strings.ToLower(field.Name)
		}
		fullKey := prefix + "." + mapKey
		if !v.IsSet(fullKey) {
			return fmt.Errorf("missing required field: %s", fullKey)
		}
	}
	return nil
}
