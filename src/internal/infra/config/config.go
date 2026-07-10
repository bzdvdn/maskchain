package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// @sk-task 01-config-bootstrap#T1.2: Create Config struct with LogConfig, mapstructure/yaml/validate tags, defaults (AC-001, AC-003)
type LogConfig struct {
	Level string `mapstructure:"level" yaml:"level" validate:"required"`
}

type Config struct {
	Log *LogConfig `mapstructure:"log" yaml:"log"`
}

const defaultLogLevel = "info"

func DefaultConfig() *Config {
	return &Config{
		Log: &LogConfig{
			Level: defaultLogLevel,
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
