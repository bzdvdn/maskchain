package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// @sk-task 01-config-bootstrap#T2.1: Implement LoadConfig with cobra root command, viper YAML/ENV/flags binding, required validation (AC-001, AC-002, AC-003, AC-005)
// NewRootCmd creates the cobra root command with config-related flags.
// Supports --config (single file), --config-dir (directory of YAML files),
// and the corresponding env var overrides CONFIG_FILE_PATH / CONFIG_DIR.
func NewRootCmd(use, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.Flags().String("config", "config.yaml", "path to config file")
	cmd.Flags().String("config-dir", "", "path to config directory (overrides --config)")
	cmd.Flags().String("log-level", "", "log level (debug, info, warn, error)")
	return cmd
}

func NewGatewayRootCmd() *cobra.Command {
	return NewRootCmd("gateway", "MaskChain AI Gateway — data plane")
}

func NewAdminRootCmd() *cobra.Command {
	return NewRootCmd("admin", "MaskChain Admin — control plane")
}

func ParseAndLoadConfig(args []string) (*Config, error) {
	cmd := NewGatewayRootCmd()
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
