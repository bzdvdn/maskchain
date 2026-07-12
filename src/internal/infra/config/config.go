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
type ProviderConfig struct {
	Name           string `mapstructure:"name" yaml:"name"`
	BaseURL        string `mapstructure:"base_url" yaml:"base_url"`
	HealthEndpoint string `mapstructure:"health_endpoint" yaml:"health_endpoint"`
	Timeout        string `mapstructure:"timeout" yaml:"timeout"`
	Priority       int    `mapstructure:"priority" yaml:"priority"`
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

// @sk-task 71-egress-streaming#T1.2: Add EgressConfig section (AC-002, AC-004, AC-006, AC-007)
type EgressConfig struct {
	MaxIdleConns    int           `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout" yaml:"idle_timeout"`
	MaxRetries      int           `mapstructure:"max_retries" yaml:"max_retries"`
	BaseBackoff     time.Duration `mapstructure:"base_backoff" yaml:"base_backoff"`
	RetryOn5xx      bool          `mapstructure:"retry_on_5xx" yaml:"retry_on_5xx"`
}

// @sk-task 80-tenant-isolation#T1.2: Add Tenants map to Config struct (AC-001, AC-003, AC-004, AC-005)
type Config struct {
	Log    *LogConfig    `mapstructure:"log" yaml:"log"`
	Server *ServerConfig `mapstructure:"server" yaml:"server"`
	DB     *DatabaseConfig `mapstructure:"database" yaml:"database"`
	Valkey *ValkeyConfig   `mapstructure:"valkey" yaml:"valkey"`
	Mask   *MaskConfig     `mapstructure:"mask" yaml:"mask"`
	Shield *ShieldConfig   `mapstructure:"shield" yaml:"shield"`
	Routing *RoutingConfig `mapstructure:"routing" yaml:"routing"`
	OTel   *OtelConfig     `mapstructure:"otel" yaml:"otel"`
	Egress *EgressConfig   `mapstructure:"egress" yaml:"egress"`
	Tenants map[string]*TenantConfig `mapstructure:"tenants" yaml:"tenants"`
}

const defaultLogLevel = "info"
const defaultPort = 8080
const defaultShutdownTimeout = 10
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
			MaxIdleConns: defaultEgressMaxIdleConns,
			IdleTimeout:  defaultEgressIdleTimeout,
			MaxRetries:   defaultEgressMaxRetries,
			BaseBackoff:  defaultEgressBaseBackoff,
			RetryOn5xx:   defaultEgressRetryOn5xx,
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
