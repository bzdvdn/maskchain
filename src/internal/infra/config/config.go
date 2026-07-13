package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// envVarReplacer maps viper key format to env var format (reverse of SetEnvKeyReplacer).
// Used to pre-populate viper from CONFIG_* env vars for Unmarshal to pick up.
var envToKeyReplacer = strings.NewReplacer("_", ".", "-", ".")

// @sk-task 01-config-bootstrap#T1.2: Create Config struct with LogConfig, mapstructure/yaml/validate tags, defaults (AC-001, AC-003)
type LogConfig struct {
	Level string `mapstructure:"level" yaml:"level" validate:"required"`
}

type ServerConfig struct {
	Port            int      `mapstructure:"port" yaml:"port"`
	ShutdownTimeout int      `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	CORSOrigins     []string `mapstructure:"cors_origins" yaml:"cors_origins"`
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

// @sk-task 51-shield-gateway-integration#T1.1: Add ShieldConfig section (AC-001, AC-002)
type ShieldConfig struct {
	ActionOnSuspicious string              `mapstructure:"action_on_suspicious" yaml:"action_on_suspicious"`
	TenantModelMapping map[string]map[string]string `mapstructure:"tenant_model_mapping" yaml:"tenant_model_mapping"`
}

// @sk-task 70-routing-engine#T1.2: Add routing config structs (AC-001, AC-002, AC-005)
// @sk-task 110-provider-adapters#T1.1: Add APIType and APIKey fields (AC-007, AC-008)
type ProviderConfig struct {
	Name           string `mapstructure:"name" yaml:"name"`
	BaseURL        string `mapstructure:"base_url" yaml:"base_url"`
	HealthEndpoint string `mapstructure:"health_endpoint" yaml:"health_endpoint"`
	Timeout        string `mapstructure:"timeout" yaml:"timeout"`
	Priority       int    `mapstructure:"priority" yaml:"priority"`
	APIType        string `mapstructure:"api_type" yaml:"api_type"`
	APIKey         string `mapstructure:"api_key" yaml:"api_key"`
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

// @sk-task 80-tenant-isolation#T1.2: Add TenantConfig struct (AC-001, AC-003, AC-004)
type TenantConfig struct {
	Name        string   `mapstructure:"name" yaml:"name"`
	ProfileSlug string   `mapstructure:"profile_slug" yaml:"profile_slug"`
	AuthHeader  string   `mapstructure:"auth_header" yaml:"auth_header"`
	AuthScheme  string   `mapstructure:"auth_scheme" yaml:"auth_scheme"`
	APIKeys     []string `mapstructure:"api_keys" yaml:"api_keys" validate:"required"`
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
type EgressConfig struct {
	MaxIdleConns        int           `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	IdleTimeout         time.Duration `mapstructure:"idle_timeout" yaml:"idle_timeout"`
	MaxRetries          int           `mapstructure:"max_retries" yaml:"max_retries"`
	BaseBackoff         time.Duration `mapstructure:"base_backoff" yaml:"base_backoff"`
	RetryOn5xx          bool          `mapstructure:"retry_on_5xx" yaml:"retry_on_5xx"`
	MaxIdleConnsPerHost int           `mapstructure:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	DisableKeepAlives   bool          `mapstructure:"disable_keep_alives" yaml:"disable_keep_alives"`
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
	DictionaryCache *DictionaryCacheConfig `mapstructure:"dictionary_cache" yaml:"dictionary_cache"`
	Tenants   map[string]*TenantConfig `mapstructure:"tenants" yaml:"tenants"`
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
const defaultDebugEnabled = false
const defaultDebugAdminToken = ""
const defaultRateLimitRate = 100
const defaultRateLimitWindowSec = 60
const defaultDictionaryCacheValkeyTTL = 300
const defaultDictionaryCacheLRUSize = 10000
const defaultDictionaryCacheWarmOnStartup = true
const defaultDictionaryCacheWarmConcurrency = 5

// @sk-task 10-gateway-skeleton#T1.2: Set ServerConfig defaults in DefaultConfig (AC-001, AC-005)
func DefaultConfig() *Config {
	return &Config{
		Log: &LogConfig{
			Level: defaultLogLevel,
		},
		Server: &ServerConfig{
			Port:            defaultPort,
			ShutdownTimeout: defaultShutdownTimeout,
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
		},
		Debug: &DebugConfig{
			Enabled:    defaultDebugEnabled,
			AdminToken: defaultDebugAdminToken,
		},
		DictionaryCache: &DictionaryCacheConfig{
			ValkeyTTLSec:    defaultDictionaryCacheValkeyTTL,
			LRUSize:         defaultDictionaryCacheLRUSize,
			WarmOnStartup:   defaultDictionaryCacheWarmOnStartup,
			WarmConcurrency: defaultDictionaryCacheWarmConcurrency,
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

func validateConfig(cfg *Config, v *viper.Viper) error {
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
