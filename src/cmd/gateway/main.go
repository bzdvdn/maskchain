package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 00-project-foundation#T1.2: Initialize project structure with empty gateway entrypoint (AC-001, AC-002, AC-006)
// @sk-task 01-config-bootstrap#T2.2: Integrate LoadConfig in main.go with zap init, debug-log (AC-004)
func main() {
	cfg := config.MustLoadConfig()

	logger, err := buildLogger(cfg.Log.Level)
	if err != nil {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Debug("config loaded", zap.Any("config", cfg))
	os.Exit(0)
}

func buildLogger(level string) (*zap.Logger, error) {
	zapCfg := zap.NewProductionConfig()
	switch level {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	return zapCfg.Build()
}
