package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"
	"go.uber.org/zap"

	"github.com/bzdvdn/maskchain/src/internal/adapters/repository/postgres"
	"github.com/bzdvdn/maskchain/src/internal/api"
	maskrepo "github.com/bzdvdn/maskchain/src/internal/adapters/repository/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	domainMask "github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task 30-shield-persistence#T2.4: Wire pool, migrations, and new repos in main
func main() {
	cfg := config.MustLoadConfig()

	logger, err := buildLogger(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Debug("config loaded", zap.Any("config", cfg))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pgPool, err := initPG(ctx, cfg.DB, logger)
	if err != nil {
		logger.Fatal("failed to init postgres", zap.Error(err))
	}

	if pgPool != nil {
		if err := postgres.RunMigrations(cfg.DB.DSN); err != nil {
			logger.Fatal("failed to run migrations", zap.Error(err))
		}
	}

	vkClient, err := initValkey(cfg.Valkey, logger)
	if err != nil {
		logger.Fatal("failed to init valkey", zap.Error(err))
	}

	registry := initDetectors(logger)

	maskTTL := time.Duration(cfg.Mask.CacheTTLSec) * time.Second
	pgRepo := maskrepo.NewPostgresMaskRepo(pgPool)
	vkRepo := maskrepo.NewValkeyMaskRepo(vkClient, maskTTL)
	maskStorage := maskrepo.NewCachedMaskRepo(pgRepo, vkRepo)
	maskUseCase := domainMask.NewMaskUseCase(registry, maskStorage)
	maskHandler := api.NewMaskHandler(maskUseCase, registry)

	srv := api.New(cfg.Server, logger)
	srv.RegisterMaskHandler(maskHandler)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutting down", zap.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("server stopped")
}

// @sk-task 22-shield-mask-storage#T5.2: Init PG with nil-safe DSN (AC-012)
// @sk-task 30-shield-persistence#T2.4: Use NewPool from postgres package (AC-005)
func initPG(ctx context.Context, dbCfg *config.DatabaseConfig, log *zap.Logger) (*pgxpool.Pool, error) {
	pool, err := postgres.NewPool(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("init pool: %w", err)
	}
	if pool == nil {
		log.Warn("no database configured, persistence disabled")
	}
	return pool, nil
}

// @sk-task 22-shield-mask-storage#T5.2: Init Valkey with nil-safe addr (AC-012)
func initValkey(vkCfg *config.ValkeyConfig, log *zap.Logger) (valkey.Client, error) {
	if vkCfg == nil || vkCfg.Addr == "" {
		log.Warn("no valkey configured, mask cache disabled")
		return nil, nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{vkCfg.Addr},
		Password:    vkCfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("valkey client: %w", err)
	}
	return client, nil
}

// @sk-task 22-shield-mask-storage#T5.2: Init detectors with CompositeDetector (AC-011)
func initDetectors(log *zap.Logger) *detector.DetectorRegistry {
	registry := detector.NewDetectorRegistry()

	pii, err := detector.NewPIIDetector()
	if err != nil {
		log.Fatal("failed to create PII detector", zap.Error(err))
	}
	secrets, err := detector.NewSecretsDetector()
	if err != nil {
		log.Fatal("failed to create secrets detector", zap.Error(err))
	}
	financial, err := detector.NewFinancialDetector()
	if err != nil {
		log.Fatal("failed to create financial detector", zap.Error(err))
	}

	combined := detector.NewCompositeDetector(pii, secrets, financial)
	if err := registry.Register(entity.DetectorTypeRegex, combined); err != nil {
		log.Fatal("register composite regex detector", zap.Error(err))
	}

	// @sk-task 24-shield-dictionaries#T5.1: Register dictionary detector type (AC-007)
	placeholder := detector.NewDictionaryDetector(nil)
	if err := registry.Register(entity.DetectorTypeDictionary, placeholder); err != nil {
		log.Fatal("register dictionary detector", zap.Error(err))
	}

	return registry
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
